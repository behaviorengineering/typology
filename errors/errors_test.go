package errors_test

import (
	"fmt"
	"testing"

	terrors "github.com/behaviorengineering/typology/errors"
)

func TestWrap_unwrapAndCode(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("disk full")
	err := terrors.Wrap(cause, terrors.CodeUnavailable, "emit.write", "write page").
		With("path", "docs/x.md")
	if err.Error() == "" {
		t.Fatal("empty error string")
	}
	if err.Unwrap() != cause {
		t.Fatalf("unwrap=%v", err.Unwrap())
	}
	code, ok := terrors.CodeOf(err)
	if !ok || code != terrors.CodeUnavailable {
		t.Fatalf("code=%q ok=%v", code, ok)
	}
	if err.Fields["path"] != "docs/x.md" {
		t.Fatalf("fields=%v", err.Fields)
	}
}

func TestWrap_nil(t *testing.T) {
	t.Parallel()
	if terrors.Wrap(nil, terrors.CodeInternal, "op", "msg") != nil {
		t.Fatal("expected nil")
	}
}
