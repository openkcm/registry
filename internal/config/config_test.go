package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/registry/internal/config"
)

func TestOrbital_ValidateTarget(t *testing.T) {
	validTarget := config.Target{
		Region: "us-west-1",
		Connection: &config.Connection{
			Type: config.ConnectionTypeAMQP,
			AMQP: &config.AMQP{
				Url:    "amqp://localhost:5672",
				Source: "source",
				Target: "target",
			},
			Auth: config.Auth{
				Type: config.AuthTypeMTLS,
				MTLS: &config.MTLS{
					CertFile: "cert.pem",
					KeyFile:  "key.pem",
					CAFile:   "ca.pem",
				},
			},
		},
	}

	tests := []struct {
		name        string
		patchTarget func(t config.Target) config.Target
		expErr      error
	}{
		{
			name:        "valid target",
			patchTarget: func(t config.Target) config.Target { return t },
			expErr:      nil,
		},
		{
			name: "valid target with none auth",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.Auth.Type = config.AuthTypeNone
				t.Connection.Auth.MTLS = nil
				return t
			},
			expErr: nil,
		},
		{
			name: "missing region",
			patchTarget: func(t config.Target) config.Target {
				t.Region = ""
				return t
			},
			expErr: config.ErrEmptyRegion,
		},
		{
			name: "missing connection",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection = nil
				return t
			},
			expErr: config.ErrNilConnection,
		},
		{
			name: "invalid connection type",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.Type = "invalid"
				return t
			},
			expErr: config.ErrUnsupportedConnectionType,
		},
		{
			name: "missing AMQP configuration",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.Type = config.ConnectionTypeAMQP
				t.Connection.AMQP = nil
				return t
			},
			expErr: config.ErrAMQPConfigMissing,
		},
		{
			name: "missing AMQP URL",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.AMQP.Url = ""
				return t
			},
			expErr: config.ErrEmptyURL,
		},
		{
			name: "missing AMQP source",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.AMQP.Source = ""
				return t
			},
			expErr: config.ErrEmptySource,
		},
		{
			name: "missing AMQP target",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.AMQP.Target = ""
				return t
			},
			expErr: config.ErrEmptyTarget,
		},
		{
			name: "invalid auth type",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.Auth.Type = "invalid"
				return t
			},
			expErr: config.ErrUnsupportedAuthType,
		},
		{
			name: "missing auth configuration for MTLS",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.Auth.Type = config.AuthTypeMTLS
				t.Connection.Auth.MTLS = nil
				return t
			},
			expErr: config.ErrNilAuth,
		},
		{
			name: "missing MTLS cert file",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.Auth.MTLS.CertFile = ""
				return t
			},
			expErr: config.ErrEmptyCertFile,
		},
		{
			name: "missing MTLS key file",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.Auth.MTLS.KeyFile = ""
				return t
			},
			expErr: config.ErrEmptyKeyFile,
		},
		{
			name: "missing MTLS CA file",
			patchTarget: func(t config.Target) config.Target {
				t = deepCopyTarget(t)
				t.Connection.Auth.MTLS.CAFile = ""
				return t
			},
			expErr: config.ErrEmptyCAFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.patchTarget(validTarget)
			c := config.Config{
				Orbital: config.Orbital{
					ConfirmJobDelay:        10 * time.Second,
					TaskLimitNum:           10,
					MaxReconcileCount:      5,
					BackoffBaseIntervalSec: 1,
					BackoffMaxIntervalSec:  10,
					Targets:                []config.Target{target},
				},
			}

			err := c.Orbital.Validate()
			if tt.expErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOrbital_ValidateWorker(t *testing.T) {
	validWorker := config.Worker{
		Name:         "confirm-job",
		NoOfWorkers:  1,
		ExecInterval: 100 * time.Millisecond,
		Timeout:      10 * time.Second,
	}

	tests := []struct {
		name        string
		patchWorker func(w config.Worker) config.Worker
		expErr      error
	}{
		{
			name:        "valid worker",
			patchWorker: func(w config.Worker) config.Worker { return w },
			expErr:      nil,
		},
		{
			name: "negative no of workers",
			patchWorker: func(w config.Worker) config.Worker {
				w.NoOfWorkers = -1
				return w
			},
			expErr: config.ErrNumberOfWorkersMustBeGreaterThanZero,
		},
		{
			name: "zero no of workers",
			patchWorker: func(w config.Worker) config.Worker {
				w.NoOfWorkers = 0
				return w
			},
			expErr: config.ErrNumberOfWorkersMustBeGreaterThanZero,
		},
		{
			name: "missing name",
			patchWorker: func(w config.Worker) config.Worker {
				w.Name = ""
				return w
			},
			expErr: config.ErrEmptyWorkerName,
		},
		{
			name: "unsupported name",
			patchWorker: func(w config.Worker) config.Worker {
				w.Name = "foo"
				return w
			},
			expErr: config.ErrUnsupportedWorkerName,
		},
		{
			name: "negative execution interval",
			patchWorker: func(w config.Worker) config.Worker {
				w.ExecInterval = -100 * time.Millisecond
				return w
			},
			expErr: config.ErrExecIntervalMustBeGreaterThanZero,
		},
		{
			name: "zero execution interval",
			patchWorker: func(w config.Worker) config.Worker {
				w.ExecInterval = 0 * time.Millisecond
				return w
			},
			expErr: config.ErrExecIntervalMustBeGreaterThanZero,
		},
		{
			name: "negative timeout",
			patchWorker: func(w config.Worker) config.Worker {
				w.Timeout = -10 * time.Second
				return w
			},
			expErr: config.ErrTimeoutMustBeGreaterThanZero,
		},
		{
			name: "zero timeout",
			patchWorker: func(w config.Worker) config.Worker {
				w.Timeout = 0 * time.Second
				return w
			},
			expErr: config.ErrTimeoutMustBeGreaterThanZero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := tt.patchWorker(validWorker)
			c := config.Config{
				Orbital: config.Orbital{
					ConfirmJobDelay:        10 * time.Second,
					TaskLimitNum:           10,
					MaxReconcileCount:      5,
					BackoffBaseIntervalSec: 1,
					BackoffMaxIntervalSec:  10,
					Workers:                []config.Worker{w},
				},
			}

			err := c.Orbital.Validate()
			if tt.expErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOrbital_ValidateFields(t *testing.T) {
	validOrbital := config.Orbital{
		ConfirmJobDelay:        10 * time.Second,
		TaskLimitNum:           10,
		MaxReconcileCount:      5,
		BackoffBaseIntervalSec: 1,
		BackoffMaxIntervalSec:  10,
	}

	tests := []struct {
		name   string
		patch  func(o config.Orbital) config.Orbital
		expErr error
	}{
		{
			name: "negative confirm job delay",
			patch: func(o config.Orbital) config.Orbital {
				o.ConfirmJobDelay = -1 * time.Second
				return o
			},
			expErr: config.ErrConfirmJobDelayMustBeEqualGreaterThanZero,
		},
		{
			name: "negative task limit num",
			patch: func(o config.Orbital) config.Orbital {
				o.TaskLimitNum = -1
				return o
			},
			expErr: config.ErrTaskLimitNumMustBeGreaterThanZero,
		},
		{
			name: "zero task limit num",
			patch: func(o config.Orbital) config.Orbital {
				o.TaskLimitNum = 0
				return o
			},
			expErr: config.ErrTaskLimitNumMustBeGreaterThanZero,
		},
		{
			name: "negative max reconcile count",
			patch: func(o config.Orbital) config.Orbital {
				o.MaxReconcileCount = -1
				return o
			},
			expErr: config.ErrMaxReconcileCountMustBeGreaterThanZero,
		},
		{
			name: "zero max reconcile count",
			patch: func(o config.Orbital) config.Orbital {
				o.MaxReconcileCount = 0
				return o
			},
			expErr: config.ErrMaxReconcileCountMustBeGreaterThanZero,
		},
		{
			name: "negative backoff base interval",
			patch: func(o config.Orbital) config.Orbital {
				o.BackoffBaseIntervalSec = -1
				return o
			},
			expErr: config.ErrBackoffBaseIntervalMustBeGreaterThanZero,
		},
		{
			name: "zero backoff base interval",
			patch: func(o config.Orbital) config.Orbital {
				o.BackoffBaseIntervalSec = 0
				return o
			},
			expErr: config.ErrBackoffBaseIntervalMustBeGreaterThanZero,
		},
		{
			name: "negative backoff max interval",
			patch: func(o config.Orbital) config.Orbital {
				o.BackoffMaxIntervalSec = -1
				return o
			},
			expErr: config.ErrBackoffMaxIntervalMustBeGreaterThanZero,
		},
		{
			name: "zero backoff max interval",
			patch: func(o config.Orbital) config.Orbital {
				o.BackoffMaxIntervalSec = 0
				return o
			},
			expErr: config.ErrBackoffMaxIntervalMustBeGreaterThanZero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := tt.patch(validOrbital)
			c := config.Config{Orbital: o}
			err := c.Orbital.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, tt.expErr)
		})
	}
}

func TestFieldValidation_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      config.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid field validation configuration",
			config: config.Config{
				Validators: config.TypeValidators{
					"type-a": config.FieldValidators{
						{
							FieldName: "field1",
							Rules: []config.ValidationRule{
								{
									Type:          "enum",
									AllowedValues: []string{"value1", "value2"},
								},
							},
						},
						{
							FieldName: "field2",
							Rules: []config.ValidationRule{
								{
									Type:          "enum",
									AllowedValues: []string{"a", "b", "c"},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty field name",
			config: config.Config{
				Validators: config.TypeValidators{
					"type-a": config.FieldValidators{
						{
							FieldName: "",
							Rules: []config.ValidationRule{
								{
									Type:          "enum",
									AllowedValues: []string{"value1", "value2"},
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "fieldName must not be empty",
		},
		{
			name: "empty allowed values in enum rule",
			config: config.Config{
				Validators: config.TypeValidators{
					"type-a": config.FieldValidators{
						{
							FieldName: "field1",
							Rules: []config.ValidationRule{
								{
									Type:          "enum",
									AllowedValues: []string{},
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "enum validation must have at least one allowed value",
		},
		{
			name: "unsupported validation type",
			config: config.Config{
				Validators: config.TypeValidators{
					"type-a": config.FieldValidators{
						{
							FieldName: "field1",
							Rules: []config.ValidationRule{
								{
									Type:          "regex",
									AllowedValues: []string{},
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "unsupported validation type: regex",
		},
		{
			name: "empty field validation array",
			config: config.Config{
				Validators: config.TypeValidators{
					"type-a": config.FieldValidators{},
				},
			},
			expectError: false,
		},
		{
			name: "empty type validation",
			config: config.Config{
				Validators: config.TypeValidators{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validators.Validate()

			if tt.expectError {
				require.Error(t, err)

				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func deepCopyTarget(t config.Target) config.Target {
	return config.Target{
		Region: t.Region,
		Connection: &config.Connection{
			Type: t.Connection.Type,
			AMQP: &config.AMQP{
				Url:    t.Connection.AMQP.Url,
				Source: t.Connection.AMQP.Source,
				Target: t.Connection.AMQP.Target,
			},
			Auth: config.Auth{
				Type: t.Connection.Auth.Type,
				MTLS: &config.MTLS{
					CertFile: t.Connection.Auth.MTLS.CertFile,
					KeyFile:  t.Connection.Auth.MTLS.KeyFile,
					CAFile:   t.Connection.Auth.MTLS.CAFile,
				},
			},
		},
	}
}
