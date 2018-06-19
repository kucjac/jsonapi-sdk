package jsonapisdk

import (
	"fmt"
	"github.com/kucjac/jsonapi"
)

type ErrorCode int

const (
	ErrNoValues ErrorCode = iota
	ErrNoModel
	ErrAlreadyWritten
	ErrInternal
	ErrValuePreset
)

type HandlerError struct {
	Code    ErrorCode
	Message string
	Scope   *jsonapi.Scope
	Field   *jsonapi.StructField
	Model   *jsonapi.ModelStruct
}

func newHandlerError(code ErrorCode, msg string) *HandlerError {
	return &HandlerError{Code: code, Message: msg}
}

func (e *HandlerError) Error() string {
	return fmt.Sprintf("%d. %s", e.Code, e.Message)
}
