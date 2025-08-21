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
	// following defer will recover from panics from the handler
	defer func() {
		rec := recover()
		if rec != nil {
			err = service.ErrPanic
			// NOTE this is to make checkmark pass
			if err != nil {
				r.logError(info.FullMethod)
			}
		}
	}()

	return handler(ctx, req)
}

// StreamInterceptor intercepts for any panics, and helps our server to recover.
// Note: It is better to add this as the last interceptor.
func (r *Recover) StreamInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	// following defer will recover from panics from the handler
	defer func() {
		rec := recover()
		if rec != nil {
			err = service.ErrPanic
			// NOTE this is to make checkmark pass
			if err != nil {
				r.logError(info.FullMethod)
			}
		}
	}()

	return handler(srv, stream)
}

// logError prints stacktrace.
func (r *Recover) logError(methodName string) {
	// we could also notify this to some notification mechanism in the future
	stackBuf := make([]byte, stackBufSize)
	stackSize := runtime.Stack(stackBuf, true)
	slog.Error(fmt.Sprintf(
		"------------------------------- \n method:[%s] \n Trace:\n %s \n--------------------------------",
		methodName,
		string(stackBuf[:stackSize])),
	)
}
