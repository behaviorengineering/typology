package errors_test

import (
	"fmt"
	"strings"
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
	got := err.Error()
	if !strings.Contains(got, "path=docs/x.md") {
		t.Fatalf("Error() missing fields: %q", got)
	}
}

func TestError_fieldsSortedAndStable(t *testing.T) {
	t.Parallel()
	err := terrors.New(terrors.CodeUnavailable, "sourceindex.listPackages", "go list failed").
		With("stderr", "missing go.sum entry").
		With("dir", "/tmp/mod")
	got := err.Error()
	want := "sourceindex.listPackages: go list failed (dir=/tmp/mod stderr=missing go.sum entry)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if terrors.FormatFields(err) != "dir=/tmp/mod stderr=missing go.sum entry" {
		t.Fatalf("FormatFields=%q", terrors.FormatFields(err))
	}
}

func TestWrap_nil(t *testing.T) {
	t.Parallel()
	if terrors.Wrap(nil, terrors.CodeInternal, "op", "msg") != nil {
		t.Fatal("expected nil")
	}
}
