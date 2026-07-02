package interceptor

import (
	"fmt"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"

	protovalidatemw "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
)

// ProtoValidation holds protovalidate interceptors for gRPC servers.
// It validates incoming requests against constraints defined in proto files.
type ProtoValidation struct {
	UnaryInterceptor  grpc.UnaryServerInterceptor
	StreamInterceptor grpc.StreamServerInterceptor
}

// NewProtoValidation creates a new ProtoValidation with initialized interceptors.
// It returns an error if the protovalidate validator cannot be initialized.
func NewProtoValidation() (ProtoValidation, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return ProtoValidation{}, fmt.Errorf("failed to create protovalidate validator: %w", err)
	}

	return ProtoValidation{
		UnaryInterceptor:  protovalidatemw.UnaryServerInterceptor(validator),
		StreamInterceptor: protovalidatemw.StreamServerInterceptor(validator),
	}, nil
}
