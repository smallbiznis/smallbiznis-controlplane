package errutil

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCCode converts the CoreStatus to its closest gRPC status code equivalent.
func (s CoreStatus) GRPCCode() codes.Code {
	switch s {
	case StatusUnauthorized:
		return codes.Unauthenticated
	case StatusForbidden:
		return codes.PermissionDenied
	case StatusNotFound:
		return codes.NotFound
	case StatusTimeout, StatusGatewayTimeout:
		return codes.DeadlineExceeded
	case StatusUnprocessableEntity:
		return codes.FailedPrecondition
	case StatusUnsupportedMediaType, StatusBadRequest, StatusValidationFailed:
		return codes.InvalidArgument
	case StatusConflict:
		return codes.AlreadyExists
	case StatusTooManyRequests:
		return codes.ResourceExhausted
	case StatusClientClosedRequest:
		return codes.Canceled
	case StatusNotImplemented:
		return codes.Unimplemented
	case StatusBadGateway, StatusServiceUnavailable:
		return codes.Unavailable
	case StatusInternal:
		return codes.Internal
	case StatusUnknown:
		return codes.Unknown
	default:
		return codes.Unknown
	}
}

// ToGRPCError normalises a domain error into a gRPC status error so handlers can
// safely return it to the transport layer.
func ToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := status.FromError(err); ok {
		return err
	}

	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}

	var base BaseError
	if errors.As(err, &base) {
		return status.Error(base.Code.GRPCCode(), base.messageWithErr())
	}

	var coder interface{ Status() CoreStatus }
	if errors.As(err, &coder) {
		return status.Error(coder.Status().GRPCCode(), err.Error())
	}

	return status.Error(codes.Internal, err.Error())
}
