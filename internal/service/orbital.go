package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"

	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	orbsql "github.com/openkcm/orbital/store/sql"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/registry/internal/config"
	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/repository"
)

var (
	ErrWrongConnectionType = errors.New("wrong initiator type")
	ErrUnexpectedJobType   = errors.New("unexpected job type")
	ErrUnexpectedStatus    = errors.New("unexpected tenant status")
)

type applyJobToTenant func(job orbital.Job, tenant *model.Tenant) error

// Orbital manages jobs and their execution targets.
type Orbital struct {
	manager *orbital.Manager
	targets map[string]orbital.Initiator
}

// NewOrbital creates a new Orbital instance.
func NewOrbital() *Orbital {
	return &Orbital{}
}

// Init initializes the Orbital manager with the provided database, repository, and target configurations.
// It sets up the AMQP clients for each target and starts the manager.
func (o *Orbital) Init(ctx context.Context, db *gorm.DB, repo repository.Repository, cfg config.Orbital) error {
	slogctx.Info(ctx, "Initializing Orbital Manager")

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB from GORM: %w", err)
	}

	store, err := orbsql.New(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("failed to create orbital store: %w", err)
	}

	orbRepo := orbital.NewRepository(store)

	targets, err := createTargets(ctx, cfg.Targets)
	if err != nil {
		return fmt.Errorf("failed to configure orbital targets: %w", err)
	}

	o.targets = targets

	manager, err := orbital.NewManager(orbRepo,
		o.resolveTasks(),
		orbital.WithTargetClients(o.targets),
		orbital.WithJobConfirmFunc(confirmJob(repo)),
		orbital.WithJobDoneEventFunc(func(ctx context.Context, job orbital.Job) error {
			return handleTerminatedJob(ctx, job, repo, applyJobDone)
		}),
		orbital.WithJobFailedEventFunc(func(ctx context.Context, job orbital.Job) error {
			return handleTerminatedJob(ctx, job, repo, applyJobAborted)
		}),
		orbital.WithJobCanceledEventFunc(func(ctx context.Context, job orbital.Job) error {
			return handleTerminatedJob(ctx, job, repo, applyJobAborted)
		}),
	)
	if err != nil {
		return fmt.Errorf("orbital manager initialization failed: %w", err)
	}

	configureOrbital(ctx, cfg, manager)

	err = manager.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start orbital job manager: %w", err)
	}

	o.manager = manager

	return nil
}

func (o *Orbital) PrepareTenantJob(ctx context.Context, tenant *model.Tenant, jobType string) error {
	ctx = slogctx.With(ctx, slog.String("jobType", jobType), slog.String("tenantID", tenant.ID.String()))

	data, err := proto.Marshal(tenant.ToProto())
	if err != nil {
		slogctx.Error(ctx, "failed to marshal job data", "error", err)
		return err
	}

	job := orbital.NewJob(jobType, data).WithExternalID(tenant.ID.String())
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

func (o *Orbital) resolveTasks() orbital.TaskResolveFunc {
	return func(ctx context.Context, job orbital.Job, _ orbital.TaskResolverCursor) (orbital.TaskResolverResult, error) {
		slogctx.Debug(ctx, "resolving tasks for job", "id", job.ID.String(), "type", job.Type)

		tenant := &tenantgrpc.Tenant{}

		err := proto.Unmarshal(job.Data, tenant)
		if err != nil {
			return orbital.TaskResolverResult{
				IsCanceled:           true,
				CanceledErrorMessage: fmt.Sprintf("failed to unmarshal tenant data: %v", err),
			}, nil
		}

		_, ok := o.targets[tenant.GetRegion()]
		if !ok {
			return orbital.TaskResolverResult{
				IsCanceled:           true,
				CanceledErrorMessage: "no orbital initiator found for region: " + tenant.GetRegion(),
			}, nil
		}

		return orbital.TaskResolverResult{
			TaskInfos: []orbital.TaskInfo{
				{
					Data:   job.Data,
					Type:   job.Type,
					Target: tenant.GetRegion(),
				},
			},
			Done: true,
		}, nil
	}
}

func confirmJob(repo repository.Repository) func(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error) {
	return func(ctx context.Context, job orbital.Job) (orbital.JobConfirmResult, error) {
		slogctx.Debug(ctx, "confirming job", "id", job.ID.String(), "type", job.Type)

		tenant, err := getTenantForJob(ctx, job, repo)
		if err != nil {
			slogctx.Error(ctx, "failed to load tenant for job", "error", err, "jobID", job.ID.String())
			return orbital.JobConfirmResult{}, err
		}

		switch job.Type {
		case tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), tenantgrpc.ACTION_ACTION_APPLY_TENANT_AUTH.String():
			return orbital.JobConfirmResult{Done: true}, nil
		case tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String(), tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String(), tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():
			status, err := jobTypeToStatus(job.Type)
			if err != nil { //nolint:nilerr // if we return an error here, the job will be retried indefinitely
				return orbital.JobConfirmResult{
					IsCanceled:           true,
					CanceledErrorMessage: fmt.Sprintf("%s: %s", ErrUnexpectedJobType, job.Type),
				}, nil
			}

			if tenant.Status != model.TenantStatus(status.String()) {
				return orbital.JobConfirmResult{}, ErrUnexpectedStatus
			}

			return orbital.JobConfirmResult{Done: true}, nil
		default:
			return orbital.JobConfirmResult{
				IsCanceled:           true,
				CanceledErrorMessage: fmt.Sprintf("%s: %s", ErrUnexpectedJobType, job.Type),
			}, nil
		}
	}
}

func handleTerminatedJob(ctx context.Context, job orbital.Job, repo repository.Repository, applyFunc applyJobToTenant) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, defaultTranTimeout)
	defer cancel()

	err := repo.Transaction(ctxTimeout, func(ctx context.Context, tx repository.Repository) error {
		tenant, err := getTenantForJob(ctx, job, tx)
		if err != nil {
			return fmt.Errorf("failed to load tenant for job: %w", err)
		}

		err = applyFunc(job, tenant)
		if err != nil {
			return fmt.Errorf("failed to apply job to tenant: %w", err)
		}

		patched, err := tx.Patch(ctx, tenant)
		if err != nil || !patched {
			return fmt.Errorf("failed to patch tenant: %w", err)
		}

		return nil
	})

	return mapError(err)
}

func applyJobDone(job orbital.Job, tenant *model.Tenant) error {
	switch job.Type {
	case tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(), tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String():
		tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()))
	case tenantgrpc.ACTION_ACTION_APPLY_TENANT_AUTH.String():
		t := &tenantgrpc.Tenant{}

		err := proto.Unmarshal(job.Data, t)
		if err != nil {
			return err
		}

		if tenant.Labels == nil {
			tenant.Labels = make(model.Labels)
		}
		maps.Copy(tenant.Labels, t.GetLabels())
	case tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String():
		tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String()))
	case tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():
		tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATED.String()))
	default:
		return fmt.Errorf("%w: %s", ErrUnexpectedJobType, job.Type)
	}

	return nil
}

func applyJobAborted(job orbital.Job, tenant *model.Tenant) error {
	switch job.Type {
	case tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String():
		tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING_ERROR.String()))
	case tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String():
		tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_UNBLOCKING_ERROR.String()))
	case tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String():
		tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKING_ERROR.String()))
	case tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():
		tenant.SetStatus(model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATION_ERROR.String()))
	default:
		return fmt.Errorf("%w: %s", ErrUnexpectedJobType, job.Type)
	}

	return nil
}

func getTenantForJob(ctx context.Context, job orbital.Job, repo repository.Repository) (*model.Tenant, error) {
	var tenant tenantgrpc.Tenant

	err := proto.Unmarshal(job.Data, &tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	tenantModel := model.Tenant{
		ID: model.ID(tenant.GetId()),
	}

	found, err := repo.Find(ctx, &tenantModel)
	if err != nil {
		return nil, ErrTenantSelect
	}

	if !found {
		return nil, ErrTenantNotFound
	}

	return &tenantModel, nil
}

func jobTypeToStatus(jobType string) (tenantgrpc.Status, error) {
	switch jobType {
	case tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String():
		return tenantgrpc.Status_STATUS_PROVISIONING, nil
	case tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String():
		return tenantgrpc.Status_STATUS_BLOCKING, nil
	case tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String():
		return tenantgrpc.Status_STATUS_UNBLOCKING, nil
	case tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String():
		return tenantgrpc.Status_STATUS_TERMINATING, nil
	default:
		return tenantgrpc.Status_STATUS_UNSPECIFIED, fmt.Errorf("%w: %s", ErrUnexpectedJobType, jobType)
	}
}
