package interceptor_test

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/openkcm/registry/internal/interceptor"
	"github.com/openkcm/registry/internal/interceptor/servicetest"
)

func TestProtoValidation(t *testing.T) {
	t.Run("NewProtoValidation", func(t *testing.T) {
		t.Run("should create a new ProtoValidation instance", func(t *testing.T) {
			// when
			pv, err := interceptor.NewProtoValidation()

			// then
			require.NoError(t, err)
			assert.NotNil(t, pv)
		})
	})

	t.Run("UnaryInterceptor", func(t *testing.T) {
		t.Run("should pass through valid request", func(t *testing.T) {
			// given
			ls := bufconn.Listen(1024 * 1024)

			serviceTest := &mockServiceTest{}
			serviceTest.FnTestCall = func(_ context.Context, in *servicetest.TestCallRequest) (*servicetest.TestCallResponse, error) {
				return &servicetest.TestCallResponse{Id: in.GetId()}, nil
			}

			pv, err := interceptor.NewProtoValidation()
			require.NoError(t, err)

			srv := grpc.NewServer(
				grpc.UnaryInterceptor(pv.UnaryInterceptor),
			)
			servicetest.RegisterTestServiceServer(srv, serviceTest)
			t.Cleanup(srv.Stop)

			go func() {
				_ = srv.Serve(ls)
			}()

			conn, err := grpc.NewClient("passthrough://bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return ls.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			defer func() { assert.NoError(t, conn.Close()) }()

			client := servicetest.NewTestServiceClient(conn)

			// when
			resp, err := client.TestCall(t.Context(), &servicetest.TestCallRequest{Id: "valid-id"})

			// then
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, "valid-id", resp.GetId())
		})

		t.Run("should reject request with empty id (min_len validation)", func(t *testing.T) {
			// given
			ls := bufconn.Listen(1024 * 1024)

			serviceTest := &mockServiceTest{}
			serviceTest.FnTestCall = func(_ context.Context, in *servicetest.TestCallRequest) (*servicetest.TestCallResponse, error) {
				return &servicetest.TestCallResponse{Id: in.GetId()}, nil
			}

			pv, err := interceptor.NewProtoValidation()
			require.NoError(t, err)

			srv := grpc.NewServer(
				grpc.UnaryInterceptor(pv.UnaryInterceptor),
			)
			servicetest.RegisterTestServiceServer(srv, serviceTest)
			t.Cleanup(srv.Stop)

			go func() {
				_ = srv.Serve(ls)
			}()

			conn, err := grpc.NewClient("passthrough://bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return ls.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			defer func() { assert.NoError(t, conn.Close()) }()

			client := servicetest.NewTestServiceClient(conn)

			// when
			resp, err := client.TestCall(t.Context(), &servicetest.TestCallRequest{Id: ""})

			// then
			assert.Nil(t, resp)
			assert.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code())
		})

		t.Run("should reject request with id exceeding max_len", func(t *testing.T) {
			// given
			ls := bufconn.Listen(1024 * 1024)

			serviceTest := &mockServiceTest{}
			serviceTest.FnTestCall = func(_ context.Context, in *servicetest.TestCallRequest) (*servicetest.TestCallResponse, error) {
				return &servicetest.TestCallResponse{Id: in.GetId()}, nil
			}

			pv, err := interceptor.NewProtoValidation()
			require.NoError(t, err)

			srv := grpc.NewServer(
				grpc.UnaryInterceptor(pv.UnaryInterceptor),
			)
			servicetest.RegisterTestServiceServer(srv, serviceTest)
			t.Cleanup(srv.Stop)

			go func() {
				_ = srv.Serve(ls)
			}()

			conn, err := grpc.NewClient("passthrough://bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return ls.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			defer func() { assert.NoError(t, conn.Close()) }()

			client := servicetest.NewTestServiceClient(conn)

			// when - id with 41 characters exceeds max_len of 40
			longID := strings.Repeat("a", 41)
			resp, err := client.TestCall(t.Context(), &servicetest.TestCallRequest{Id: longID})

			// then
			assert.Nil(t, resp)
			assert.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code())
		})
	})

	t.Run("StreamInterceptor", func(t *testing.T) {
		t.Run("should pass through valid stream request", func(t *testing.T) {
			// given
			ls := bufconn.Listen(1024 * 1024)

			serviceTest := &mockServiceTest{}
			serviceTest.FnTestCallStream = func(in *servicetest.TestCallRequest, resp grpc.ServerStreamingServer[servicetest.TestCallResponse]) error {
				return resp.Send(&servicetest.TestCallResponse{Id: in.GetId()})
			}

			pv, err := interceptor.NewProtoValidation()
			require.NoError(t, err)

			srv := grpc.NewServer(
				grpc.StreamInterceptor(pv.StreamInterceptor),
			)
			servicetest.RegisterTestServiceServer(srv, serviceTest)
			t.Cleanup(srv.Stop)

			go func() {
				_ = srv.Serve(ls)
			}()

			conn, err := grpc.NewClient("passthrough://bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return ls.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			defer func() { assert.NoError(t, conn.Close()) }()

			client := servicetest.NewTestServiceClient(conn)

			// when
			stream, err := client.TestCallStream(t.Context(), &servicetest.TestCallRequest{Id: "valid-stream-id"})

			// then
			require.NoError(t, err)
			resp, err := stream.Recv()
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, "valid-stream-id", resp.GetId())
		})

		t.Run("should reject stream request with empty id", func(t *testing.T) {
			// given
			ls := bufconn.Listen(1024 * 1024)

			serviceTest := &mockServiceTest{}
			serviceTest.FnTestCallStream = func(in *servicetest.TestCallRequest, resp grpc.ServerStreamingServer[servicetest.TestCallResponse]) error {
				return resp.Send(&servicetest.TestCallResponse{Id: in.GetId()})
			}

			pv, err := interceptor.NewProtoValidation()
			require.NoError(t, err)

			srv := grpc.NewServer(
				grpc.StreamInterceptor(pv.StreamInterceptor),
			)
			servicetest.RegisterTestServiceServer(srv, serviceTest)
			t.Cleanup(srv.Stop)

			go func() {
				_ = srv.Serve(ls)
			}()

			conn, err := grpc.NewClient("passthrough://bufnet",
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return ls.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			defer func() { assert.NoError(t, conn.Close()) }()

			client := servicetest.NewTestServiceClient(conn)

			// when
			stream, err := client.TestCallStream(t.Context(), &servicetest.TestCallRequest{Id: ""})

			// then - the error may come from starting the stream or from Recv
			if err != nil {
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, codes.InvalidArgument, st.Code())
			} else {
				_, err = stream.Recv()
				assert.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, codes.InvalidArgument, st.Code())
			}
		})
	})
}
