package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type (
	ConnectionType string
	AuthType       string
)

const (
	ConnectionTypeAMQP ConnectionType = "amqp"
)

const (
	AuthTypeMTLS AuthType = "mtls"
	AuthTypeNone AuthType = "none"
)

const (
	WorkerNameConfirmJob  = "confirm-job"
	WorkerNameCreateTask  = "create-task"
	WorkerNameReconcile   = "reconcile"
	WorkerNameNotifyEvent = "notify-event"
)

var (
	ErrEmptyRegion               = errors.New("region must not be empty")
	ErrNilConnection             = errors.New("connection configuration is missing")
	ErrUnsupportedConnectionType = errors.New("connection type is not supported")
	ErrNilAuth                   = errors.New("authentication configuration is missing")
	ErrUnsupportedAuthType       = errors.New("authentication type is not supported")
	// AMQP specific errors.
	ErrAMQPConfigMissing = errors.New("AMQP configuration is missing")
	ErrEmptyURL          = errors.New("URL must not be empty")
	ErrEmptySource       = errors.New("source must not be empty")
	ErrEmptyTarget       = errors.New("target must not be empty")
	// MTLS specific errors.
	ErrEmptyCAFile   = errors.New("CA file must not be empty")
	ErrEmptyCertFile = errors.New("certificate file must not be empty")
	ErrEmptyKeyFile  = errors.New("key file must not be empty")
	// Worker specific errors.
	ErrEmptyWorkerName                      = errors.New("worker name must not be empty")
	ErrExecIntervalMustBeGreaterThanZero    = errors.New("worker exec interval must be greater than zero")
	ErrUnsupportedWorkerName                = errors.New("worker name is not supported, please use one of the predefined worker names (confirm-job, create-task, reconcile, notify-event)")
	ErrNumberOfWorkersMustBeGreaterThanZero = errors.New("number of workers must be greater than zero")
	ErrTimeoutMustBeGreaterThanZero         = errors.New("timeout must be greater than zero")
	// Orbital specific errors.
	ErrConfirmJobAfterMustBeEqualGreaterThanZero = errors.New("confirm job delay must be equal or greater than zero")
	ErrTaskLimitNumMustBeGreaterThanZero         = errors.New("task limit number must be greater than zero")
	ErrMaxReconcileCountMustBeGreaterThanZero    = errors.New("max reconcile count must be greater than zero")
	ErrBackoffBaseIntervalMustBeGreaterThanZero  = errors.New("backoff base interval must be greater than zero")
	ErrBackoffMaxIntervalMustBeGreaterThanZero   = errors.New("backoff max interval must be greater than zero")
)

// Config holds all application configuration parameters.
type Config struct {
	commoncfg.BaseConfig `mapstructure:",squash"`

	// gRPC server configuration
	GRPCServer GRPCServer `yaml:"grpcServer"`
	// Database configuration
	Database DB `yaml:"database" json:"database"`
	// Orbital configuration
	Orbital Orbital `yaml:"orbital" json:"orbital"`
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	return c.Orbital.Validate()
}

// DB holds DB config.
type DB struct {
	Host     string              `yaml:"host" json:"host"`
	User     commoncfg.SourceRef `yaml:"user" json:"user"`
	Password commoncfg.SourceRef `yaml:"password" json:"password"`
	Name     string              `yaml:"name" json:"name"` // database name
	Port     string              `yaml:"port" json:"port"`
}

// Server holds server config.
type Server struct {
	Port string `yaml:"port"`
}

// GRPCServer configuration.
type GRPCServer struct {
	commoncfg.GRPCServer `mapstructure:",squash"`

	// also embed client attributes for the gRPC health check client
	Client commoncfg.GRPCClient `yaml:"client" json:"client"`
}

type Orbital struct {
	ConfirmJobAfter        time.Duration `yaml:"confirmJobAfter" json:"confirmJobAfter"`
	TaskLimitNum           int           `yaml:"taskLimitNum" json:"taskLimitNum"`
	MaxReconcileCount      int64         `yaml:"maxReconcileCount" json:"maxReconcileCount"`
	BackoffBaseIntervalSec int64         `yaml:"backoffBaseIntervalSec" json:"backoffBaseIntervalSec"`
	BackoffMaxIntervalSec  int64         `yaml:"backoffMaxIntervalSec" json:"backoffMaxIntervalSec"`
	Targets                []Target      `yaml:"targets" json:"targets"`
	Workers                []Worker      `yaml:"workers" json:"workers"`
}

func (o *Orbital) Validate() error {
	if o.ConfirmJobAfter < 0 {
		return fmt.Errorf("%w: %v", ErrConfirmJobAfterMustBeEqualGreaterThanZero, o.ConfirmJobAfter)
	}

	if o.TaskLimitNum <= 0 {
		return fmt.Errorf("%w: %d", ErrTaskLimitNumMustBeGreaterThanZero, o.TaskLimitNum)
	}

	if o.MaxReconcileCount <= 0 {
		return fmt.Errorf("%w: %d", ErrMaxReconcileCountMustBeGreaterThanZero, o.MaxReconcileCount)
	}

	if o.BackoffBaseIntervalSec <= 0 {
		return fmt.Errorf("%w: %d", ErrBackoffBaseIntervalMustBeGreaterThanZero, o.BackoffBaseIntervalSec)
	}

	if o.BackoffMaxIntervalSec <= 0 {
		return fmt.Errorf("%w: %d", ErrBackoffMaxIntervalMustBeGreaterThanZero, o.BackoffMaxIntervalSec)
	}

	for _, target := range o.Targets {
		err := target.validate()
		if err != nil {
			return fmt.Errorf("invalid target configuration: %w", err)
		}
	}

	for _, worker := range o.Workers {
		err := worker.validate()
		if err != nil {
			return fmt.Errorf("invalid worker configuration for %s: %w", worker.Name, err)
		}
	}

	return nil
}

func (o *Orbital) GetWorker(workerName string) *Worker {
	for _, worker := range o.Workers {
		if worker.Name == workerName {
			return &worker
		}
	}

	return nil
}

type Target struct {
	Region     string      `yaml:"region" json:"region"`
	Connection *Connection `yaml:"connection" json:"connection"`
}

func (t *Target) validate() error {
	if t.Region == "" {
		return ErrEmptyRegion
	}

	if t.Connection == nil {
		return fmt.Errorf("%w, target %s", ErrNilConnection, t.Region)
	}

	err := t.Connection.validate()
	if err != nil {
		return fmt.Errorf("invalid connection configuration for target %s: %w", t.Region, err)
	}

	return nil
}

type Worker struct {
	Name         string        `yaml:"name" json:"name"`
	NoOfWorkers  int           `yaml:"noOfWorkers" json:"noOfWorkers"`
	ExecInterval time.Duration `yaml:"execInterval" json:"execInterval"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
}

func (w *Worker) validate() error {
	if w.NoOfWorkers <= 0 {
		return ErrNumberOfWorkersMustBeGreaterThanZero
	}

	if w.ExecInterval <= 0 {
		return ErrExecIntervalMustBeGreaterThanZero
	}

	if w.Timeout <= 0 {
		return ErrTimeoutMustBeGreaterThanZero
	}

	switch w.Name {
	case WorkerNameConfirmJob, WorkerNameCreateTask, WorkerNameReconcile, WorkerNameNotifyEvent:
		return nil
	case "":
		return ErrEmptyWorkerName
	default:
		return ErrUnsupportedWorkerName
	}
}

type Connection struct {
	Type ConnectionType `yaml:"type" json:"type"`
	AMQP *AMQP          `yaml:"amqp" json:"amqp"`
	Auth Auth           `yaml:"auth" json:"auth"`
}

func (c *Connection) validate() error {
	switch c.Type {
	case ConnectionTypeAMQP:
		if c.AMQP == nil {
			return ErrAMQPConfigMissing
		}

		err := c.AMQP.validate()
		if err != nil {
			return fmt.Errorf("invalid AMQP configuration: %w", err)
		}
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedConnectionType, c.Type)
	}

	return c.Auth.validate()
}

type AMQP struct {
	Url    string `yaml:"url" json:"url"`
	Source string `yaml:"source" json:"source"`
	Target string `yaml:"target" json:"target"`
}

func (a *AMQP) validate() error {
	if a.Url == "" {
		return ErrEmptyURL
	}

	if a.Source == "" {
		return ErrEmptySource
	}

	if a.Target == "" {
		return ErrEmptyTarget
	}

	return nil
}

type Auth struct {
	Type AuthType `yaml:"type" json:"type"`
	MTLS *MTLS    `yaml:"mtls" json:"mtls"`
}

func (a *Auth) validate() error {
	switch a.Type {
	case AuthTypeMTLS:
		if a.MTLS == nil {
			return ErrNilAuth
		}

		return a.MTLS.validate()
	case AuthTypeNone:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedAuthType, a.Type)
	}
}

type MTLS struct {
	CAFile   string `yaml:"caFile" json:"caFile"`
	CertFile string `yaml:"certFile" json:"certFile"`
	KeyFile  string `yaml:"keyFile" json:"keyFile"`
}

func (m *MTLS) validate() error {
	if m.CAFile == "" {
		return ErrEmptyCAFile
	}

	if m.CertFile == "" {
		return ErrEmptyCertFile
	}

	if m.KeyFile == "" {
		return ErrEmptyKeyFile
	}

	return nil
}
