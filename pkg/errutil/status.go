package errutil

import (
	"net/http"

	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const HTTPStatusClientClosedRequest = 499

type CoreStatus metav1.StatusReason

const (
	StatusUnknown CoreStatus = CoreStatus(metav1.StatusReasonUnknown)

	StatusUnauthorized CoreStatus = CoreStatus(metav1.StatusReasonUnauthorized)

	StatusForbidden CoreStatus = CoreStatus(metav1.StatusReasonForbidden)

	StatusBadRequest CoreStatus = CoreStatus(metav1.StatusReasonBadRequest)

	StatusNotFound CoreStatus = CoreStatus(metav1.StatusReasonNotFound)

	StatusTimeout CoreStatus = CoreStatus(metav1.StatusReasonTimeout)

	StatusServiceUnavailable CoreStatus = CoreStatus(metav1.StatusReasonServiceUnavailable)

	StatusUnsupportedMediaType CoreStatus = CoreStatus(metav1.StatusReasonUnsupportedMediaType)

	StatusUnprocessableEntity CoreStatus = CoreStatus("Unprocessable Entity")

	StatusConflict CoreStatus = CoreStatus(metav1.StatusReasonConflict)

	StatusTooManyRequests CoreStatus = CoreStatus(metav1.StatusReasonTooManyRequests)

	StatusClientClosedRequest CoreStatus = CoreStatus("Client closed request")

	StatusNotImplemented CoreStatus = CoreStatus("Not implemented")

	StatusBadGateway CoreStatus = CoreStatus("Bad gateway")

	StatusGatewayTimeout CoreStatus = CoreStatus("Gateway timeout")

	StatusInternal CoreStatus = CoreStatus(metav1.StatusReasonInternalError)

	StatusValidationFailed CoreStatus = CoreStatus("Validation failed")
)

func (s CoreStatus) Status() CoreStatus {
	return s
}

func (s CoreStatus) HTTPStatus() int {
	switch s {
	case StatusUnauthorized:
		return http.StatusUnauthorized
	case StatusForbidden:
		return http.StatusForbidden
	case StatusNotFound:
		return http.StatusNotFound
	case StatusTimeout, StatusGatewayTimeout:
		return http.StatusGatewayTimeout
	case StatusUnprocessableEntity:
		return http.StatusUnprocessableEntity
	case StatusUnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	case StatusConflict:
		return http.StatusConflict
	case StatusTooManyRequests:
		return http.StatusTooManyRequests
	case StatusBadRequest, StatusValidationFailed:
		return http.StatusBadRequest
	case StatusClientClosedRequest:
		return HTTPStatusClientClosedRequest
	case StatusNotImplemented:
		return http.StatusNotImplemented
	case StatusBadGateway:
		return http.StatusBadGateway
	case StatusUnknown, StatusInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

func (s CoreStatus) LogLevel() zapcore.Level {
	switch s {
	case StatusUnauthorized:
		return zapcore.InfoLevel
	case StatusForbidden:
		return zapcore.InfoLevel
	case StatusNotFound:
		return zapcore.InfoLevel
	case StatusTimeout:
		return zapcore.InfoLevel
	case StatusUnsupportedMediaType:
		return zapcore.InfoLevel
	case StatusUnprocessableEntity:
		return zapcore.InfoLevel
	case StatusConflict:
		return zapcore.InfoLevel
	case StatusTooManyRequests:
		return zapcore.InfoLevel
	case StatusBadRequest:
		return zapcore.InfoLevel
	case StatusValidationFailed:
		return zapcore.InfoLevel
	case StatusNotImplemented:
		return zapcore.DebugLevel
	case StatusUnknown, StatusInternal:
		return zapcore.ErrorLevel
	default:
		return zapcore.Level(zapcore.UnknownType)
	}
}

func (s CoreStatus) String() string {
	return string(s)
}
