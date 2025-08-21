package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/status"

	"github.com/openkcm/registry/internal/service"
)

var errSomething = errors.New("error something")

func TestMapError(t *testing.T) {
	// given
	tts := []struct {
		name   string
		input  error
		expOut error
	}{
		{
			name:   "should return nil",
			input:  nil,
			expOut: nil,
		},
		{
			name:   "should return same error if error is not mapped",
			input:  errSomething,
			expOut: errSomething,
		},
		{
			name:   "should return transaction aborted error if context DeadlineExceeded",
			input:  context.DeadlineExceeded,
			expOut: service.ErrTranCtxTimeout,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// when
			result := service.MapError(tt.input)

			// then
			assert.Equal(t, tt.expOut, result)
		})
	}
}

func TestErrorWithParams(t *testing.T) {
	// given
	testCases := []struct {
		desc        string
		inputError  error
		inputParams []any
		expErrorMsg string
	}{
		{
			desc:        "should create error with param as nil",
			inputError:  service.ErrKeyClaimAlreadyActive,
			inputParams: nil,
			expErrorMsg: "rpc error: code = FailedPrecondition desc = key claim is already active",
		},
		{
			desc:        "should create error with empty param",
			inputError:  service.ErrKeyClaimAlreadyActive,
			inputParams: []any{},
			expErrorMsg: "rpc error: code = FailedPrecondition desc = key claim is already active",
		},
		{
			desc:        "should create error with only one param",
			inputError:  service.ErrKeyClaimAlreadyActive,
			inputParams: []any{"foo"},
			expErrorMsg: "rpc error: code = FailedPrecondition desc = key claim is already active (foo)",
		},
		{
			desc:        "should create error with 2 param",
			inputError:  service.ErrKeyClaimAlreadyActive,
			inputParams: []any{"foo", "bar"},
			expErrorMsg: "rpc error: code = FailedPrecondition desc = key claim is already active (foo=bar)",
		},
		{
			desc:        "should create error with 3 param",
			inputError:  service.ErrKeyClaimAlreadyActive,
			inputParams: []any{"foo", "bar", "baz"},
			expErrorMsg: "rpc error: code = FailedPrecondition desc = key claim is already active (foo=bar baz)",
		},
		{
			desc:        "should create error with array",
			inputError:  service.ErrKeyClaimAlreadyActive,
			inputParams: []any{"systemID", []string{"foo", "baz"}},
			expErrorMsg: "rpc error: code = FailedPrecondition desc = key claim is already active (systemID=[foo baz])",
		},
		{
			desc:        "should create error with map",
			inputError:  service.ErrKeyClaimAlreadyActive,
			inputParams: []any{"query", map[string]string{"foo": "baz"}},
			expErrorMsg: "rpc error: code = FailedPrecondition desc = key claim is already active (query=map[foo:baz])",
		},
		{
			desc:        "should create a non GRPC error with map",
			inputError:  errSomething,
			inputParams: []any{"query", map[string]string{"foo": "baz"}},
			expErrorMsg: "error something (query=map[foo:baz])",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			// when
			result := service.ErrorWithParams(tC.inputError, tC.inputParams...)

			// then
			assert.Equal(t, tC.expErrorMsg, result.Error())

			sts, isGRPCErr := status.FromError(tC.inputError)
			if isGRPCErr {
				assert.Equal(t, sts.Code(), status.Code(result))
			} else {
				assert.ErrorIs(t, result, tC.inputError)
			}
		})
	}
}
