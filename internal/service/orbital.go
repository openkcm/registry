package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"gorm.io/gorm"

	orbsql "github.com/openkcm/orbital/store/sql"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/config"
)

var (
	ErrWrongConnectionType = errors.New("wrong initiator type")
	ErrUnexpectedJobType   = errors.New("unexpected job type")
)

type (
	// Orbital manages jobs and their execution targets.
	Orbital struct {
		manager  *orbital.Manager
		targets  map[string]orbital.Initiator
		registry handlerRegistry
	}

	handlerRegistry struct {
		mu sync.RWMutex
		r  map[string]JobHandler
	}
)

type (
	// JobSource provides job identification and encoded job data.
	JobSource interface {
		IDString() string
		ProtoBytes() ([]byte, error)
	}

	// JobHandler defines the lifecycle callbacks for job processing.
	JobHandler interface {
		ConfirmJob(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error)
		ResolveTasks(ctx context.Context, job orbital.Job, targets map[string]orbital.Initiator) (orbital.TaskResolverResult, error)
		ApplyJobDone(ctx context.Context, job orbital.Job) error
		ApplyJobAborted(ctx context.Context, job orbital.Job) error
	}
)

// NewOrbital initializes the Orbital manager with the provided database and target configurations.
// It sets up the AMQP clients for each target and starts the manager.
func NewOrbital(ctx context.Context, db *gorm.DB, cfg config.Orbital) (*Orbital, error) {
	slogctx.Info(ctx, "Initializing Orbital Manager")

	sqlDB, err := db.DB()
	if err != nil {
		return &Orbital{}, fmt.Errorf("failed to get SQL DB from GORM: %w", err)
	}

	store, err := orbsql.New(ctx, sqlDB)
	if err != nil {
		return &Orbital{}, fmt.Errorf("failed to create orbital store: %w", err)
	}
	orbRepo := orbital.NewRepository(store)

	targets, err := createTargets(ctx, cfg.Targets)
	if err != nil {
		return &Orbital{}, fmt.Errorf("failed to configure orbital targets: %w", err)
	}
	o := &Orbital{
		targets: targets,
	}

	manager, err := orbital.NewManager(orbRepo,
		o.resolveTasks(),
		orbital.WithTargetClients(targets),
		orbital.WithJobConfirmFunc(o.confirmJob()),
		orbital.WithJobDoneEventFunc(o.handleJobDone()),
		orbital.WithJobFailedEventFunc(o.handleJobAborted()),
		orbital.WithJobCanceledEventFunc(o.handleJobAborted()),
	)
	if err != nil {
		return &Orbital{}, fmt.Errorf("orbital manager initialization failed: %w", err)
	}

	configureOrbital(ctx, cfg, manager)

	err = manager.Start(ctx)
	if err != nil {
		return &Orbital{}, fmt.Errorf("failed to start orbital job manager: %w", err)
	}

	o.manager = manager
	return o, nil
}

func (o *Orbital) RegisterJobHandler(jobType string, handler JobHandler) {
	o.registry.mu.Lock()
	defer o.registry.mu.Unlock()

	if o.registry.r == nil {
		o.registry.r = make(map[string]JobHandler)
	}

	o.registry.r[jobType] = handler
}

func (o *Orbital) PrepareJob(ctx context.Context, source JobSource, jobType string) error {
	ctx = slogctx.With(ctx, slog.String("job type", jobType), slog.String("external ID", source.IDString()))

	data, err := source.ProtoBytes()
	if err != nil {
		slogctx.Error(ctx, "failed to get job data bytes", "error", err)
		return fmt.Errorf("failed to get job data bytes: %w", err)
	}

	job := orbital.NewJob(jobType, data).WithExternalID(source.IDString())
	job, err = o.manager.PrepareJob(ctx, job)
	if err != nil {
		slogctx.Error(ctx, "failed to prepare job", "error", err)
		return err
	}

	slogctx.Debug(ctx, "Job prepared", "jobId", job.ID)
	return nil
}

func createTargets(ctx context.Context, cfgTargets []config.Target) (map[string]orbital.Initiator, error) {
	targets := make(map[string]orbital.Initiator, len(cfgTargets))
	for _, cfgTarget := range cfgTargets {
		slogctx.Info(ctx, "creating orbital target", slog.String("Region", cfgTarget.Region))

		client, err := createAMQPClient(ctx, cfgTarget)
		if err != nil {
			return nil, fmt.Errorf("failed to create AMQP client for %s: %w", cfgTarget.Region, err)
		}

		targets[cfgTarget.Region] = client
	}

	return targets, nil
}

func createAMQPClient(ctx context.Context, cfgTarget config.Target) (*amqp.AMQP, error) {
	if cfgTarget.Connection.Type != config.ConnectionTypeAMQP {
		return nil, fmt.Errorf("%w: %s", ErrWrongConnectionType, cfgTarget.Connection.Type)
	}

	connInfo := amqp.ConnectionInfo{
		URL:    cfgTarget.Connection.AMQP.Url,
		Target: cfgTarget.Connection.AMQP.Target,
		Source: cfgTarget.Connection.AMQP.Source,
	}

	var option amqp.ClientOption

	switch cfgTarget.Connection.Auth.Type {
	case config.AuthTypeMTLS:
		option = amqp.WithExternalMTLS(
			cfgTarget.Connection.Auth.MTLS.CertFile,
			cfgTarget.Connection.Auth.MTLS.KeyFile,
			cfgTarget.Connection.Auth.MTLS.CAFile,
			"",
		)
	case config.AuthTypeNone:
		option = amqp.WithNoAuth()
	default:
		return nil, fmt.Errorf("%w: %s", config.ErrUnsupportedAuthType, cfgTarget.Connection.Auth.Type)
	}

	client, err := amqp.NewClient(ctx, &codec.Proto{}, connInfo, option)
	if err != nil {
		return nil, err
	}

	slogctx.Info(ctx, "created orbital AMQP client",
		slog.String("URL", connInfo.URL),
		slog.String("Target", connInfo.Target),
		slog.String("Source", connInfo.Source),
		slog.String("AuthType", string(cfgTarget.Connection.Auth.Type)),
	)

	return client, nil
}

func configureOrbital(ctx context.Context, cfg config.Orbital, manager *orbital.Manager) {
	manager.Config.ConfirmJobAfter = cfg.ConfirmJobAfter
	manager.Config.TaskLimitNum = cfg.TaskLimitNum
	manager.Config.MaxReconcileCount = cfg.MaxReconcileCount
	manager.Config.BackoffBaseIntervalSec = cfg.BackoffBaseIntervalSec
	manager.Config.BackoffMaxIntervalSec = cfg.BackoffMaxIntervalSec
	configureOrbitalWorkers(ctx, cfg, manager)
	slogctx.Info(ctx, "effective orbital configuration", "config", manager.Config)
}

func configureOrbitalWorkers(ctx context.Context, cfg config.Orbital, manager *orbital.Manager) {
	configureOrbitalWorker(ctx, cfg.GetWorker(config.WorkerNameCreateTask), &manager.Config.CreateTasksWorkerConfig)
	configureOrbitalWorker(ctx, cfg.GetWorker(config.WorkerNameConfirmJob), &manager.Config.ConfirmJobWorkerConfig)
	configureOrbitalWorker(ctx, cfg.GetWorker(config.WorkerNameReconcile), &manager.Config.ReconcileWorkerConfig)
	configureOrbitalWorker(ctx, cfg.GetWorker(config.WorkerNameNotifyEvent), &manager.Config.NotifyWorkerConfig)
}

func configureOrbitalWorker(ctx context.Context, cfg *config.Worker, worker *orbital.WorkerConfig) {
	if cfg == nil {
		return
	}

	if cfg.NoOfWorkers > 0 {
		worker.NoOfWorkers = cfg.NoOfWorkers
	}

	if cfg.ExecInterval > 0 {
		worker.ExecInterval = cfg.ExecInterval
	}

	if cfg.Timeout > 0 {
		worker.Timeout = cfg.Timeout
	}

	slogctx.Info(ctx, "configured orbital worker", "name", cfg.Name, "worker", worker)
}

func (o *Orbital) confirmJob() orbital.JobConfirmFunc {
	return func(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error) {
		slogctx.Debug(ctx, "confirming job", "id", job.ID.String(), "type", job.Type, "externalID", job.ExternalID)

		h, ok := o.getHandler(ctx, job.Type)
		if !ok {
			return orbital.JobConfirmResult{
				IsCanceled:           true,
				CanceledErrorMessage: fmt.Sprintf("%s: %s", ErrUnexpectedJobType, job.Type),
			}, nil
		}

		return h.ConfirmJob(ctx, job)
	}
}

func (o *Orbital) resolveTasks() orbital.TaskResolveFunc {
	return func(ctx context.Context, job orbital.Job, cursor orbital.TaskResolverCursor) (orbital.TaskResolverResult, error) {
		slogctx.Debug(ctx, "resolving tasks for job", "id", job.ID.String(), "type", job.Type, "externalID", job.ExternalID)

		h, ok := o.getHandler(ctx, job.Type)
		if !ok {
			return orbital.TaskResolverResult{
				IsCanceled:           true,
				CanceledErrorMessage: fmt.Sprintf("%s: %s", ErrUnexpectedJobType, job.Type),
			}, nil
		}

		return h.ResolveTasks(ctx, job, o.targets)
	}
}

func (o *Orbital) handleJobDone() orbital.JobTerminatedEventFunc {
	return func(ctx context.Context, job orbital.Job) error {
		slogctx.Debug(ctx, "handling done job", "id", job.ID.String(), "type", job.Type, "externalID", job.ExternalID)

		h, ok := o.getHandler(ctx, job.Type)
		if !ok {
			return nil
		}

		return h.ApplyJobDone(ctx, job)
	}
}

func (o *Orbital) handleJobAborted() orbital.JobTerminatedEventFunc {
	return func(ctx context.Context, job orbital.Job) error {
		slogctx.Debug(ctx, "handling aborted job", "id", job.ID.String(), "type", job.Type, "externalID", job.ExternalID)

		h, ok := o.getHandler(ctx, job.Type)
		if !ok {
			return nil
		}

		return h.ApplyJobAborted(ctx, job)
	}
}

func (o *Orbital) getHandler(ctx context.Context, jobType string) (JobHandler, bool) {
	o.registry.mu.RLock()
	defer o.registry.mu.RUnlock()

	h, ok := o.registry.r[jobType]
	if !ok {
		slogctx.Error(ctx, "no job handler registered", slog.String("jobType", jobType))
	}

	return h, ok
}
