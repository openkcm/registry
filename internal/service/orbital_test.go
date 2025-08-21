package service_test

import (
	"fmt"
	"testing"

	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/model"
	"github.com/openkcm/registry/internal/service"
)

func TestApplyJobDone(t *testing.T) {
	tests := []struct {
		name      string
		job       orbital.Job
		tenant    *model.Tenant
		expTenant model.Tenant
		expErr    error
	}{
		{
			name: "Provisioning job",
			job: orbital.Job{
				Type: string(service.ProvisionTenant),
			},
			tenant: &model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING.String()),
			},
			expTenant: model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()),
			},
			expErr: nil,
		},
		{
			name: "Blocking job",
			job: orbital.Job{
				Type: string(service.BlockTenant),
			},
			tenant: &model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKING.String()),
			},
			expTenant: model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKED.String()),
			},
			expErr: nil,
		},
		{
			name: "Unblocking job",
			job: orbital.Job{
				Type: string(service.UnblockTenant),
			},
			tenant: &model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_UNBLOCKING.String()),
			},
			expTenant: model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_ACTIVE.String()),
			},
			expErr: nil,
		},
		{
			name: "Terminating job",
			job: orbital.Job{
				Type: string(service.TerminateTenant),
			},
			tenant: &model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATING.String()),
			},
			expTenant: model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATED.String()),
			},
			expErr: nil,
		},
		{
			name: "Unexpected job type",
			job: orbital.Job{
				Type: "unexpected_job_type",
			},
			expErr: fmt.Errorf("%w: %s", service.ErrUnexpectedJobType, "unexpected_job_type"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ApplyJobDone(tt.job, tt.tenant)
			if tt.expErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expTenant.Status, tt.tenant.Status)
			}
		})
	}
}

func TestApplyJobAborted(t *testing.T) {
	tests := []struct {
		name      string
		job       orbital.Job
		tenant    *model.Tenant
		expTenant model.Tenant
		expErr    error
	}{
		{
			name: "Provisioning job",
			job: orbital.Job{
				Type: string(service.ProvisionTenant),
			},
			tenant: &model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING.String()),
			},
			expTenant: model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_PROVISIONING_ERROR.String()),
			},
			expErr: nil,
		},
		{
			name: "Blocking job",
			job: orbital.Job{
				Type: string(service.BlockTenant),
			},
			tenant: &model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKING.String()),
			},
			expTenant: model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_BLOCKING_ERROR.String()),
			},
			expErr: nil,
		},
		{
			name: "Unblocking job",
			job: orbital.Job{
				Type: string(service.UnblockTenant),
			},
			tenant: &model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_UNBLOCKING.String()),
			},
			expTenant: model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_UNBLOCKING_ERROR.String()),
			},
			expErr: nil,
		},
		{
			name: "Terminating job",
			job: orbital.Job{
				Type: string(service.TerminateTenant),
			},
			tenant: &model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATING.String()),
			},
			expTenant: model.Tenant{
				Status: model.TenantStatus(tenantgrpc.Status_STATUS_TERMINATION_ERROR.String()),
			},
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ApplyJobAborted(tt.job, tt.tenant)
			if tt.expErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expTenant.Status, tt.tenant.Status)
			}
		})
	}
}
