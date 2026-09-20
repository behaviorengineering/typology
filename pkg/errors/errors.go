// Package errors provides typed domain errors for typology.
package errors

import (
	stderrors "errors"
	"fmt"
	"sort"
	"strings"
)

// Code is a stable machine-readable error class.
type Code string

const (
	CodeInvalid            Code = "invalid"
	CodeNotFound           Code = "not_found"
	CodeFailedPrecondition Code = "failed_precondition"
	CodeUnavailable        Code = "unavailable"
	CodeInternal           Code = "internal"
)

// Error is a typology domain error with Unwrap for the cause chain.
type Error struct {
	Code   Code
	Op     string
	Msg    string
	Fields map[string]string
	Cause  error
}

// Error implements the error interface.
// Attached Fields (via With) are appended so CLI %v output surfaces
// diagnostics such as go list stderr without a separate formatter.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	if e.Op != "" {
		b.WriteString(e.Op)
		b.WriteString(": ")
	}
	b.WriteString(e.Msg)
	if e.Cause != nil {
		b.WriteString(": ")
		b.WriteString(e.Cause.Error())
	}
	if fields := FormatFields(e); fields != "" {
		b.WriteString(" (")
		b.WriteString(fields)
		b.WriteString(")")
	}
	return b.String()
}

// Unwrap returns the cause.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// With attaches a field for troubleshooting and returns the same error.
func (e *Error) With(key, value string) *Error {
	if e == nil {
		return nil
	}
	if e.Fields == nil {
		e.Fields = make(map[string]string, 1)
	}
	e.Fields[key] = value
	return e
}

// New returns a domain error without a cause.
func New(code Code, op, msg string) *Error {
	return &Error{Code: code, Op: op, Msg: msg}
}

// Wrap wraps cause in a domain error. If cause is nil, returns nil.
func Wrap(cause error, code Code, op, msg string) *Error {
	if cause == nil {
		return nil
	}
	return &Error{Code: code, Op: op, Msg: msg, Cause: cause}
}

// CodeOf returns the domain Code if err unwraps to *Error.
func CodeOf(err error) (Code, bool) {
	var de *Error
	if stderrors.As(err, &de) {
		return de.Code, true
	}
	return "", false
}

// FormatFields returns fields as stable "k=v" pairs for logs and Error().
func FormatFields(e *Error) string {
	if e == nil || len(e.Fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, e.Fields[k]))
	}
	return strings.Join(parts, " ")
}
