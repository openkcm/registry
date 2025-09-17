// Package operatortest provides a test operator for the registry service.
//
// It connects to the AMQP broker as defined in the config.yaml.
// It handles certain tenant IDs with specific responses:
// - Tenants with ID "test-tenant-success" will get a successful handler response.
// - Tenants with ID "test-tenant-fail" will get a failed handler response.
package operatortest

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/openkcm/orbital/codec"
	"google.golang.org/protobuf/proto"

	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"

	"github.com/openkcm/registry/internal/config"
)

var ErrNoTestRegion = errors.New("no test region found in configuration")

func New(ctx context.Context) (*orbital.Operator, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	var target *config.Target

	for _, t := range cfg.Orbital.Targets {
		if t.Region == "test-region" {
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
		URL:    target.Connection.AMQP.Url,
		Target: target.Connection.AMQP.Source,
		Source: target.Connection.AMQP.Target,
	}, option)
	if err != nil {
		return nil, err
	}

	operator, err := orbital.NewOperator(client)
	if err != nil {
		return nil, err
	}

	for _, jobType := range []string{
		tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String(),
		tenantgrpc.ACTION_ACTION_APPLY_TENANT_AUTH.String(),
		tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String(),
		tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String(),
		tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String(),
	} {
		err = operator.RegisterHandler(jobType, handle)
		if err != nil {
			return nil, err
		}
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

func handle(_ context.Context, handlerReq orbital.HandlerRequest) (orbital.HandlerResponse, error) {
	var tenant tenantgrpc.Tenant

	err := proto.Unmarshal(handlerReq.Data, &tenant)
	if err != nil {
		return orbital.HandlerResponse{}, err
	}

	switch tenant.GetId() {
	case "test-tenant-success":
		return orbital.HandlerResponse{
			Result: orbital.ResultDone,
		}, nil
	case "test-tenant-fail":
		return orbital.HandlerResponse{
			Result: orbital.ResultFailed,
		}, nil
	}

	return orbital.HandlerResponse{
		Result: orbital.ResultProcessing,
	}, nil
}
