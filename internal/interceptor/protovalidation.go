package interceptor

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"

	protovalidatemw "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
)

// ProtoValidation wraps a protovalidate validator for gRPC interceptors.
// It validates incoming requests against constraints defined in proto files.
type ProtoValidation struct {
	unaryInterceptor  grpc.UnaryServerInterceptor
	streamInterceptor grpc.StreamServerInterceptor
}

// NewProtoValidation creates a new ProtoValidation interceptor.
// It returns an error if the protovalidate validator cannot be initialized.
func NewProtoValidation() (*ProtoValidation, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create protovalidate validator: %w", err)
	}

	return &ProtoValidation{
		unaryInterceptor:  protovalidatemw.UnaryServerInterceptor(validator),
		streamInterceptor: protovalidatemw.StreamServerInterceptor(validator),
	}, nil
}

// UnaryInterceptor validates unary RPC requests against proto constraints.
// It returns codes.InvalidArgument if validation fails.
func (p *ProtoValidation) UnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	return p.unaryInterceptor(ctx, req, info, handler)
}

// StreamInterceptor validates streaming RPC requests against proto constraints.
// It returns codes.InvalidArgument if validation fails.
func (p *ProtoValidation) StreamInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return p.streamInterceptor(srv, stream, info, handler)
}
