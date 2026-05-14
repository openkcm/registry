// Package operatortest provides a test operator for the registry service.
//
// It connects to the AMQP broker as defined in the config.yaml.
//
// It handles certain tenant IDs with specific responses:
// - Tenants with ID "test-tenant-success" will get a successful handler response.
// - Tenants with ID "test-tenant-fail" will get a failed handler response.
//
// It handles certain auth external IDs with specific responses:
// - Auths with External ID "test-auth-success" will get a successful handler response.
// - Auths with External ID "test-auth-fail" will get a failed handler response.
//
// For any other tenant IDs or auth external IDs, it will return a processing response.
package operatortest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"google.golang.org/protobuf/proto"

	authgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/auth/v1"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/config"
)

const Region = "test-region"

const (
	TenantIDFail    = "test-tenant-fail"
	TenantIDSuccess = "test-tenant-success"
)

const (
	AuthExternalIDFail    = "test-auth-fail"
	AuthExternalIDSuccess = "test-auth-success"
	taskWaitTime          = 1 * time.Second
)

var ErrNoTestRegion = errors.New("no test region found in configuration")

func New(ctx context.Context) (*orbital.Operator, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	var target *config.Target
	for _, t := range cfg.Orbital.Targets {
		if t.Region == Region {
			target = &t
			break
		}
	}
	if target == nil {
		return nil, ErrNoTestRegion
	}

	option, err := getClientOption(target)
	if err != nil {
		return nil, err
	}

	client, err := amqp.NewClient(ctx, codec.Proto{}, amqp.ConnectionInfo{
		URL:    target.Connection.AMQP.URL,
		Target: target.Connection.AMQP.Source,
		Source: target.Connection.AMQP.Target,
	}, option)
	if err != nil {
		return nil, err
	}

	operatorTarget := orbital.TargetOperator{
		Client: client,
	}

	operator, err := orbital.NewOperator(operatorTarget)
	if err != nil {
		return nil, err
	}

	err = registerHandlers(operator)
	if err != nil {
		return nil, err
	}

	return operator, nil
}

func getClientOption(target *config.Target) (amqp.ClientOption, error) {
	var option amqp.ClientOption

	switch target.Connection.Auth.Type {
	case config.AuthTypeMTLS:
		option = amqp.WithExternalMTLS(
			"../local/rabbitmq/certs/client.crt",
			"../local/rabbitmq/certs/client.key",
			"../local/rabbitmq/certs/ca.crt",
			"",
		)
	case config.AuthTypeNone:
		option = amqp.WithNoAuth()
	default:
		return nil, fmt.Errorf("%w: %s", config.ErrUnsupportedAuthType, target.Connection.Auth.Type)
	}

	return option, nil
}

func loadConfig() (*config.Config, error) {
	cfg := &config.Config{}
	loader := commoncfg.NewLoader(cfg,
		commoncfg.WithPaths(
			"./",
			"./.."),
		commoncfg.WithEnvOverride(""))

	err := loader.LoadConfig()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func registerHandlers(operator *orbital.Operator) error {
	for _, jobType := range []string{
		tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(),
		tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String(),
		tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String(),
		tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String(),
	} {
		err := operator.RegisterHandler(jobType, handleTenant)
		if err != nil {
			return err
		}
	}

	for _, jobType := range []string{
		authgrpc.AuthAction_AUTH_ACTION_APPLY_AUTH.String(),
		authgrpc.AuthAction_AUTH_ACTION_REMOVE_AUTH.String(),
	} {
		err := operator.RegisterHandler(jobType, handleAuth)
		if err != nil {
			return err
		}
	}
	return nil
}

func handleTenant(_ context.Context,
	handlerRequest orbital.HandlerRequest,
	handlerResponse *orbital.HandlerResponse) {
	var tenant tenantgrpc.Tenant

	err := proto.Unmarshal(handlerRequest.TaskData, &tenant)
	if err != nil {
		handlerResponse.Fail(fmt.Sprintf("failed to unmarshal tenant: %v", err))
		return
	}

	switch tenant.GetId() {
	case TenantIDSuccess:
		handlerResponse.Complete()
	case TenantIDFail:
		handlerResponse.Fail("simulated tenant failure")
	default:
		handlerResponse.ContinueAndWaitFor(taskWaitTime)
	}
}

func handleAuth(_ context.Context,
	handlerRequest orbital.HandlerRequest,
	handlerResponse *orbital.HandlerResponse) {
	var auth authgrpc.Auth

	err := proto.Unmarshal(handlerRequest.TaskData, &auth)
	if err != nil {
		handlerResponse.Fail(fmt.Sprintf("failed to unmarshal auth: %v", err))
		return
	}

	switch auth.GetExternalId() {
	case AuthExternalIDSuccess:
		handlerResponse.Complete()
	case AuthExternalIDFail:
		handlerResponse.Fail("simulated auth failure")
	default:
		handlerResponse.ContinueAndWaitFor(taskWaitTime)
	}
}
