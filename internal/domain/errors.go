package domain

import "fmt"

type ErrorKind string

const (
	KindInvalid  ErrorKind = "invalid_argument"
	KindNotFound ErrorKind = "not_found"
	KindConflict ErrorKind = "conflict"
	KindGate     ErrorKind = "gate_failed"
	KindCorrupt  ErrorKind = "integrity_error"
)

type BusinessError struct {
	Kind    ErrorKind `json:"kind"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
}

func (e *BusinessError) Error() string { return e.Message }

func NewError(kind ErrorKind, code, format string, args ...any) error {
	return &BusinessError{Kind: kind, Code: code, Message: fmt.Sprintf(format, args...)}
}

func Invalid(code, format string, args ...any) error {
	return NewError(KindInvalid, code, format, args...)
}

func Gate(code, format string, args ...any) error {
	return NewError(KindGate, code, format, args...)
}

func Conflict(code, format string, args ...any) error {
	return NewError(KindConflict, code, format, args...)
}

func NotFound(entity, id string) error {
	return NewError(KindNotFound, "not_found", "%s %s 不存在", entity, id)
}
