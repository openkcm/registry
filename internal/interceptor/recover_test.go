package interceptor_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/openkcm/registry/internal/interceptor"
	"github.com/openkcm/registry/internal/interceptor/servicetest"
	"github.com/openkcm/registry/internal/service"
)

// Service implements the procedure calls defined as protobufs.
type mockServiceTest struct {
	servicetest.UnimplementedTestServiceServer

	FnTestCall       func(context.Context, *servicetest.TestCallRequest) (*servicetest.TestCallResponse, error)
	FnTestCallStream func(*servicetest.TestCallRequest, grpc.ServerStreamingServer[servicetest.TestCallResponse]) error
}

func (m *mockServiceTest) TestCall(ctx context.Context, r *servicetest.TestCallRequest) (*servicetest.TestCallResponse, error) {
	return m.FnTestCall(ctx, r)
}

func (m *mockServiceTest) TestCallStream(r *servicetest.TestCallRequest, s grpc.ServerStreamingServer[servicetest.TestCallResponse]) error {
	return m.FnTestCallStream(r, s)
}

func TestServerPanic(t *testing.T) {
	t.Run("Recover", func(t *testing.T) {
		t.Run("UnaryInterceptor should make server recover from panic and return a valid error message to client", func(t *testing.T) {
			// given
			// creating a buffered listener
			ls := bufconn.Listen(1024 * 1024)

			serviceTest := &mockServiceTest{}
			serviceTest.FnTestCall = func(_ context.Context, in *servicetest.TestCallRequest) (*servicetest.TestCallResponse, error) {
				if in.GetId() == "panic" {
					panic("I like to panic here")
				}

				return &servicetest.TestCallResponse{Id: "success"}, nil
			}

			srv := grpc.NewServer(
				// making server with recover interceptor.
				grpc.UnaryInterceptor(interceptor.NewRecover().UnaryInterceptor),
			)
			// registering server
			servicetest.RegisterTestServiceServer(srv, serviceTest)

			go func(t *testing.T, srv *grpc.Server, ls *bufconn.Listener) {
				t.Helper()

				defer srv.Stop()

				err := srv.Serve(ls)
				if err != nil {
					assert.NoError(t, err, "server could not be started")
				}
			}(t, srv, ls)

			// creating client connection
			conn, err := grpc.NewClient("passthrough://bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return ls.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			defer func() {
				assert.NoError(t, conn.Close())
			}()

			// creating client
			client := servicetest.NewTestServiceClient(conn)

			// when
			resp, err := client.TestCall(
				t.Context(),
				&servicetest.TestCallRequest{Id: "panic"},
			)

			// then
			assert.Nil(t, resp)
			assert.Equal(t, service.ErrPanic.Error(), err.Error())

			// when
			// this call is make sure that server is still running
			resp2, err2 := client.TestCall(
				t.Context(),
				&servicetest.TestCallRequest{},
			)

			// then
			assert.NotNil(t, resp2)
			assert.Equal(t, "success", resp2.GetId())
			assert.NoError(t, err2)
		})

		t.Run("StreamInterceptor should make server recover from panic and return a valid error message to client", func(t *testing.T) {
			// given
			// creating a buffered listener
			ls := bufconn.Listen(1024 * 1024)

			serviceTest := &mockServiceTest{}
			serviceTest.FnTestCallStream = func(in *servicetest.TestCallRequest, resp grpc.ServerStreamingServer[servicetest.TestCallResponse]) error {
				if in.GetId() == "panic" {
					panic("I like to panic here")
				}

				return resp.Send(&servicetest.TestCallResponse{Id: "success"})
			}

			srv := grpc.NewServer(
				// making server with recover interceptor.
				grpc.StreamInterceptor(interceptor.NewRecover().StreamInterceptor),
			)

			// registering server
			servicetest.RegisterTestServiceServer(srv, serviceTest)

			go func(t *testing.T, srv *grpc.Server, ls *bufconn.Listener) {
				t.Helper()

				defer srv.Stop()

				err := srv.Serve(ls)
				if err != nil {
					assert.NoError(t, err, "server could not be started")
				}
			}(t, srv, ls)

			// creating client connection
			conn, err := grpc.NewClient("passthrough://bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return ls.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)

			defer func() {
				assert.NoError(t, conn.Close())
			}()

			// creating client
			client := servicetest.NewTestServiceClient(conn)

			// when
			strm, err := client.TestCallStream(
				t.Context(),
				&servicetest.TestCallRequest{Id: "panic"},
			)

			// then
			assert.NoError(t, err)

			resp, err := strm.Recv()
			assert.Nil(t, resp)
			assert.Equal(t, service.ErrPanic.Error(), err.Error())

			// when
			// this call is make sure that server is still running
			strm2, err2 := client.TestCallStream(
				t.Context(),
				&servicetest.TestCallRequest{},
			)

			// then
			assert.NoError(t, err2)

			resp2, err2 := strm2.Recv()
			assert.NotNil(t, resp2)
			assert.Equal(t, "success", resp2.GetId())
			assert.NoError(t, err2)
		})
	})
}

func TestUnaryInterceptor(t *testing.T) {
	ctx := t.Context()
	t.Run("should recover and return error if there is a panic from handler func", func(t *testing.T) {
		// given
		handlerFunc := func(context.Context, any) (any, error) {
			panic("yes i want to panic here")
		}

		subj := interceptor.NewRecover()

		// when
		res, err := subj.UnaryInterceptor(
			ctx,
			"req",
			&grpc.UnaryServerInfo{FullMethod: ""},
			handlerFunc,
		)

		// then
		assert.Equal(t, service.ErrPanic, err)
		assert.Nil(t, res)
	})

	t.Run("should return successfully from handler func if there are no panic", func(t *testing.T) {
		// given
		expResult := "foo"
		handlerFunc := func(context.Context, any) (any, error) {
			return expResult, nil
		}
		subj := interceptor.NewRecover()

		// when
		res, err := subj.UnaryInterceptor(
			ctx,
			"req",
			&grpc.UnaryServerInfo{FullMethod: ""},
			handlerFunc,
		)

		// then
		assert.NoError(t, err)
		assert.Equal(t, expResult, res)
	})
}

func TestStreamInterceptor(t *testing.T) {
	t.Run("should recover and return error if there is a panic from handler func", func(t *testing.T) {
		// given
		handlerFunc := func(any, grpc.ServerStream) error {
			panic("yes i want to panic here")
		}
		subj := interceptor.NewRecover()

		// when
		err := subj.StreamInterceptor(
			"any",
			nil,
			&grpc.StreamServerInfo{FullMethod: ""},
			handlerFunc,
		)

		// then
		assert.Equal(t, service.ErrPanic, err)
	})

	t.Run("should return successfully from handler func if there are no panic", func(t *testing.T) {
		// given
		handlerFunc := func(any, grpc.ServerStream) error {
			return nil
		}
		subj := interceptor.NewRecover()

		// when
		err := subj.StreamInterceptor(
			"any",
			nil,
			&grpc.StreamServerInfo{FullMethod: ""},
			handlerFunc,
		)

		// then
		assert.NoError(t, err)
	})
}
