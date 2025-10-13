package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	SelectTenantErrMsg      = "could not select tenant"
	UpdateTenantErrMsg      = "could not update tenant"
	DeleteTenantErrMsg      = "could not delete tenant"
	TenantNotFoundMsg       = "tenant not found"
	TenantUnavailableErrMsg = "tenant is unavailable"
)

const (
	SelectSystemErrMsg                  = "could not select system"
	UpdateSystemErrMsg                  = "could not update system"
	DeleteSystemErrMsg                  = "could not delete system"
	SystemNotFoundMsg                   = "system not found"
	SystemUnavailableErrMsg             = "system is unavailable"
	TenantStatusTransitionNotAllowedMsg = "tenant status transition not allowed"
	InvalidTenantStatusMsg              = "invalid tenant status"
)

const (
	SelectAuthErrMsg     = "could not select auth"
	UpdateAuthErrMsg     = "could not update auth"
	AuthNotFoundErrMsg   = "auth not found"
	AuthAlreadyExistsMsg = "auth with the given external ID already exists"
	AuthInvalidStatusMsg = "invalid auth status"
)

const (
	MissingLabelKeysMsg = "missing label keys"
	MissingLabelsMsg    = "missing labels"
	EmptyLabelKeysMsg   = "label keys cannot be empty"
)

var (
	ErrTenantSelect                     = status.Error(codes.Internal, SelectTenantErrMsg)
	ErrTenantUpdate                     = status.Error(codes.Internal, UpdateTenantErrMsg)
	ErrTenantDelete                     = status.Error(codes.Internal, DeleteTenantErrMsg)
	ErrTenantIDFormat                   = status.Error(codes.InvalidArgument, "tenant ID is not valid")
	ErrTenantNotFound                   = status.Error(codes.NotFound, TenantNotFoundMsg)
	ErrTenantUnavailable                = status.Error(codes.FailedPrecondition, TenantUnavailableErrMsg)
	ErrTenantEncoding                   = status.Error(codes.Internal, "failed to encode tenant data")
	ErrTenantStatusTransitionNotAllowed = errors.New(TenantStatusTransitionNotAllowedMsg)
	ErrInvalidTenantStatus              = errors.New(InvalidTenantStatusMsg)
)

var (
	ErrSystemSelect              = status.Error(codes.Internal, SelectSystemErrMsg)
	ErrSystemUpdate              = status.Error(codes.Internal, UpdateSystemErrMsg)
	ErrSystemDelete              = status.Error(codes.Internal, DeleteSystemErrMsg)
	ErrExternalIDIsEmpty         = status.Error(codes.InvalidArgument, "external ID cannot be empty")
	ErrRegionIsEmpty             = status.Error(codes.InvalidArgument, "region cannot be empty")
	ErrSystemNotFound            = status.Error(codes.NotFound, SystemNotFoundMsg)
	ErrSystemIsLinkedToTenant    = status.Error(codes.FailedPrecondition, "system is linked to the tenant")
	ErrSystemIsNotLinkedToTenant = status.Error(codes.FailedPrecondition, "system is not linked to the tenant")
	ErrSystemHasL1KeyClaim       = status.Error(codes.FailedPrecondition, "system has active l1 key claim")
	ErrSystemUnavailable         = status.Error(codes.FailedPrecondition, SystemUnavailableErrMsg)
	ErrNoSystemIdentifiers       = status.Error(codes.InvalidArgument, "no system identifiers provided")
	ErrSystemListNotAllowed      = status.Error(codes.InvalidArgument, "need either externalID and region or tenantID to list systems")
)

var (
	ErrAuthSelect        = status.Error(codes.Internal, SelectAuthErrMsg)
	ErrAuthUpdate        = status.Error(codes.Internal, UpdateAuthErrMsg)
	ErrAuthNotFound      = status.Error(codes.NotFound, AuthNotFoundErrMsg)
	ErrAuthAlreadyExists = status.Error(codes.AlreadyExists, AuthAlreadyExistsMsg)
	ErrAuthInvalidStatus = status.Error(codes.FailedPrecondition, AuthInvalidStatusMsg)
)

var (
	ErrTranCtxTimeout          = status.Error(codes.Aborted, "transaction was aborted due to timeout, please try again")
	ErrPanic                   = status.Error(codes.Internal, "an unexpected error occurred on the server, please try again")
	ErrKeyClaimAlreadyActive   = status.Error(codes.FailedPrecondition, "key claim is already active")
	ErrKeyClaimAlreadyInactive = status.Error(codes.FailedPrecondition, "key claim is already inactive")
	ErrMissingLabelKeys        = status.Error(codes.InvalidArgument, MissingLabelKeysMsg)
	ErrMissingLabels           = status.Error(codes.InvalidArgument, MissingLabelsMsg)
	ErrEmptyLabelKeys          = status.Error(codes.InvalidArgument, EmptyLabelKeysMsg)
)

// ErrorWithParams will return an error with new message,
// where params get appended at end of the error message.
// If the input is normal error then error is wrapped.
// If the input is a GRPC error it will create a new
// GRPC error.
// Note GRPC error returned cannot be used to check `errors.Is`.
func ErrorWithParams(err error, params ...any) error {
	var sb strings.Builder

	if len(params) == 0 {
		return err
	}

	for index, param := range params {
		if (index+1)%2 == 0 {
			sb.WriteString(fmt.Sprintf("=%v", param))
		} else {
			if index != 0 {
				sb.WriteString(" ")
			}

			sb.WriteString(fmt.Sprintf("%v", param))
		}
	}

	suffix := ""
	if sb.Len() > 0 {
		suffix = fmt.Sprintf(" (%s)", sb.String())
	}

	sts, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w%s", err, suffix)
	}

	return status.Error(sts.Code(), sts.Message()+suffix)
}

// mapError maps an error to a corresponding error.
// if err == context.DeadlineExceeded returns ErrTranCtxTimeout.
// else return input error.
func mapError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return ErrTranCtxTimeout
	default:
		return err
	}
}
