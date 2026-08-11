package interceptor

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"

	"google.golang.org/grpc"

	"github.com/openkcm/registry/internal/service"
)

const stackBufSize = 9 << 11

// Recover helps in recovering panics grpc endpoints.
// we could also add a client to notify in the future.
type Recover struct{}

// NewRecover will create a Recover instance.
// Recover as both Unary  and Stream interceptor for server.
// More information about the interceptors can be found here.
// https://grpc.io/docs/guides/interceptors
func NewRecover() *Recover {
	return &Recover{}
}

// UnaryInterceptor intercepts for any panics, and helps our server to recover.
// Note: It is better to add this as the last interceptor.
func (r *Recover) UnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, err error) {
	defer func() {
		if p := recover(); p != nil {
			r.logError(info.FullMethod, p)
			err = service.ErrPanic
		}
	}()

	return handler(ctx, req)
}

// StreamInterceptor intercepts for any panics, and helps our server to recover.
// Note: It is better to add this as the last interceptor.
func (r *Recover) StreamInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if p := recover(); p != nil {
			r.logError(info.FullMethod, p)
			err = service.ErrPanic
		}
	}()

	return handler(srv, stream)
}

// logError prints the panic value and stacktrace.
func (r *Recover) logError(methodName string, panicValue any) {
	// we could also notify this to some notification mechanism in the future
	stackBuf := make([]byte, stackBufSize)
	stackSize := runtime.Stack(stackBuf, true)
	slog.Error(fmt.Sprintf(
		"------------------------------- \n method:[%s] \n panic: %v \n Trace:\n %s \n--------------------------------",
		methodName,
		panicValue,
		string(stackBuf[:stackSize])),
	)
}
