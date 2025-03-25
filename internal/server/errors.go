package server

import (
	"fmt"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HttpError struct {
	Code    int
	Message string
}

func (e *HttpError) Error() string {
	return e.Message
}

func (e *HttpError) StatusCode() int {
	return e.Code
}

func (e *HttpError) ToAPIStatus() *metav1.Status {
	return &metav1.Status{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Status",
			APIVersion: "v1",
		},
		Status:  metav1.StatusFailure,
		Code:    int32(e.Code),
		Reason:  metav1.StatusReasonUnknown,
		Message: e.Message,
	}
}

func (e *HttpError) IsNil() bool {
	return e == nil
}

func NewHttpError(code int, format string, a ...any) *HttpError {
	message := fmt.Sprintf(format, a...)
	return &HttpError{
		Code:    code,
		Message: message,
	}
}

func NewNotFoundError(format string, a ...any) *HttpError {
	return NewHttpError(http.StatusNotFound, format, a...)
}

func NewBadRequestError(format string, a ...any) *HttpError {
	return NewHttpError(http.StatusBadRequest, format, a...)
}

func NewInternalServerError(format string, a ...any) *HttpError {
	return NewHttpError(http.StatusInternalServerError, format, a...)
}
