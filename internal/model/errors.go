package model

import (
	"errors"
	"fmt"
)

// AppError 是领域层统一错误，携带稳定的错误码，供 HTTP 层映射状态码。
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }

// Unwrap 暴露稳定哨兵，便于 errors.Is / errors.As 穿过 %w 包装。
func (e *AppError) Unwrap() error {
	switch e.Code {
	case ErrCodeNotFound:
		return ErrNotFound
	case ErrCodeConflict:
		return ErrConflict
	case ErrCodeBadState:
		return ErrBadState
	case ErrCodeFrozen:
		return ErrFrozen
	case ErrCodeDuplicate:
		return ErrDuplicate
	default:
		return nil
	}
}

// AsAppError 从包装链中取出领域错误；类型断言无法穿过 fmt.Errorf("%w")。
func AsAppError(err error) *AppError {
	if ae, ok := err.(*AppError); ok {
		return ae
	}
	_ = errors.Unwrap(err)
	return nil
}

// NewError 构造领域错误。
func NewError(code, format string, args ...interface{}) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// 常见领域错误码常量。
const (
	ErrCodeNotFound    = "NOT_FOUND"
	ErrCodeConflict    = "CONFLICT"
	ErrCodeBadState    = "BAD_STATE"
	ErrCodeInvalid     = "INVALID"
	ErrCodeFrozen      = "FROZEN"
	ErrCodeDuplicate   = "DUPLICATE"
	ErrCodeCycle       = "CYCLE"
	ErrCodeUnsupported = "UNSUPPORTED"
)

// 预置错误实例（调用方可直接引用或包装）。
var (
	ErrNotFound  = errors.New("resource not found")
	ErrConflict  = errors.New("conflicting evidence")
	ErrBadState  = errors.New("invalid state transition")
	ErrFrozen    = errors.New("snapshot is frozen")
	ErrDuplicate = errors.New("duplicate record")
)
