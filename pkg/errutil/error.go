package errutil

import (
	"fmt"
	"net/url"
	"strings"
)

type Detail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type BaseError struct {
	Code    CoreStatus `json:"code"`
	Message string     `json:"message"`
	Details []Detail   `json:"details,omitempty"`
	Err     error      `json:"-"`
}

func (e BaseError) Status() CoreStatus {
	return e.Code
}

func (e BaseError) URL() string {
	values := url.Values{}

	values.Set("error_code", string(e.Code))
	values.Set("error_message", e.Message)

	for _, d := range e.Details {
		values.Set("details["+strings.TrimSpace(d.Field)+"]", d.Message)
	}

	return values.Encode()
}

func (e BaseError) JSON() interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"code":    e.Code,
			"message": e.messageWithErr(),
			"details": e.Details,
		},
	}
}

func (e BaseError) Unwrap() error {
	return e.Err
}

func (e BaseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s", e.Code, e.messageWithErr())
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e BaseError) messageWithErr() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

type Option func(*BaseError)

func WithDetails(details ...Detail) Option {
	return func(be *BaseError) { be.Details = details }
}

func WithErr(err error) Option {
	return func(be *BaseError) { be.Err = err }
}

func New(code CoreStatus, message string, opts ...Option) error {
	be := BaseError{Code: code, Message: message}
	for _, opt := range opts {
		opt(&be)
	}
	return be
}

func NotFound(msg string, err error, options ...Option) error {
	return New(StatusNotFound, msg, options...)
}

func UnprocessableEntity(msg string, err error, options ...Option) error {
	return New(StatusUnprocessableEntity, msg, options...)
}

func UnsupportedMediaType(msg string, err error, options ...Option) error {
	return New(StatusUnsupportedMediaType, msg, options...)
}

func Conflict(msg string, err error, options ...Option) error {
	return New(StatusConflict, msg, options...)
}

func BadRequest(msg string, err error, options ...Option) error {
	return New(StatusBadRequest, msg, options...)
}

func ValidationFailed(msg string, err error, options ...Option) error {
	return New(StatusValidationFailed, msg, options...)
}

func Internal(msg string, err error, options ...Option) error {
	return New(StatusInternal, msg, options...)
}

func Timeout(msg string, err error, options ...Option) error {
	return New(StatusTimeout, msg, options...)
}

func Unauthorized(msg string, err error, options ...Option) error {
	return New(StatusUnauthorized, msg, options...)
}

func Forbidden(msg string, err error, options ...Option) error {
	return New(StatusForbidden, msg, options...)
}

func TooManyRequest(msg string, err error, options ...Option) error {
	return New(StatusTooManyRequests, msg, options...)
}

func ClientClosedRequest(msg string, err error, options ...Option) error {
	return New(StatusClientClosedRequest, msg, options...)
}

func NotImplemented(msg string, err error, options ...Option) error {
	return New(StatusNotImplemented, msg, options...)
}

func BadGateway(msg string, err error, options ...Option) error {
	return New(StatusBadGateway, msg, options...)
}
