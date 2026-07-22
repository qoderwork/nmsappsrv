package dto

import (
	"nmsappsrv/pkg/enums"
)

// Result mirrors the Java envelope com.waveoss.common.vo.base.Result<T>
// EXACTLY — field names (`code`, `msg`, `data`, `exend`, `description`),
// default values (code=200, msg="success"), and the extra `exend` HashMap for
// ad-hoc fields. This wire contract is consumed by the original frontend as-is.
//
// NOTE: the Java class deliberately misspells the field as `exend` (not
// `extend`). We keep the same spelling on the wire for JSON compatibility.
type Result[T any] struct {
	Code        int                    `json:"code"`
	Msg         string                 `json:"msg"`
	Data        T                      `json:"data,omitempty"`
	Description string                 `json:"description,omitempty"`
	Exend       map[string]interface{} `json:"exend"`
}

// NewResult builds a Result with a zero-value T. Always allocates the Exend
// HashMap so callers can `.Put()` without nil checks — mirrors Java default
// constructor behaviour.
func NewResult[T any]() *Result[T] {
	var zero T
	return &Result[T]{
		Code:  200,
		Msg:   "success",
		Data:  zero,
		Exend: make(map[string]interface{}),
	}
}

// ---- Static constructors (mirror Result.ok / Result.failure in Java) ----

func OK() *Result[any] {
	return NewResult[any]()
}

func OKMsg(msg string) *Result[any] {
	r := NewResult[any]()
	if msg != "" {
		r.Msg = msg
	}
	return r
}

func OKData[T any](data T) *Result[T] {
	r := NewResult[T]()
	r.Data = data
	return r
}

func Failure(code int, msg string) *Result[any] {
	r := NewResult[any]()
	r.Code = code
	r.Msg = msg
	return r
}

func FailureStatus(status enums.Status) *Result[any] {
	return Failure(status.Code, status.Message)
}

func FailureInvalidArgument() *Result[any] {
	return Failure(enums.BAD_REQUEST.Code, "INVALID_ARGUMENT")
}

// ---- Mutators ----

// Put adds a key/value to the Exend HashMap — mirrors Java Result.put(k,v).
func (r *Result[T]) Put(key string, value interface{}) *Result[T] {
	if r.Exend == nil {
		r.Exend = make(map[string]interface{})
	}
	r.Exend[key] = value
	return r
}

// SetData sets the primary payload — mirrors Java Result.setData(data).
func (r *Result[T]) SetData(data T) *Result[T] {
	r.Data = data
	return r
}
