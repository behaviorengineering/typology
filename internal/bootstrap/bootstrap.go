// Package bootstrap configures Typology as a Go tool in a consumer module.
package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/gorepo"
)

// ToolPackage is the command package declared by consumers as a Go tool.
const ToolPackage = "github.com/behaviorengineering/typology/cmd/typology"

// DefaultToolVersion is the first released version supported by the bootstrap.
const DefaultToolVersion = "v0.0.5"

// CommandRunner runs one Go command in a module directory.
type CommandRunner func(ctx context.Context, dir string, args []string, stdout, stderr io.Writer) error

// Options configures Typology tool bootstrap.
type Options struct {
	RepoRoot   string
	Module     string
	Version    string
	Stdout     io.Writer
	Stderr     io.Writer
	RunCommand CommandRunner
}

// Result identifies the module configured by Run.
type Result struct {
	Module  gorepo.Module
	Version string
}

// Run declares the Typology CLI as a tool, tidies the module, and verifies it.
func Run(ctx context.Context, opts Options) (Result, error) {
	if ctx == nil {
		return Result{}, terrors.New(terrors.CodeInvalid, "bootstrap.Run", "context is nil")
	}
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		return Result{}, terrors.New(terrors.CodeInvalid, "bootstrap.Run", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeInvalid, "bootstrap.Run", "resolve repo root").
			With("repo", repo)
	}
	version, err := toolVersion(opts.Version)
	if err != nil {
		return Result{}, err
	}
	modules, err := gorepo.Modules(absRepo)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeFailedPrecondition, "bootstrap.Run", "resolve consumer modules").
			With("repo", absRepo)
	}
	module, err := selectModule(modules, opts.Module)
	if err != nil {
		return Result{}, err
	}

	runner := opts.RunCommand
	if runner == nil {
		runner = runGoCommand
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	toolRef := ToolPackage + "@" + version
	if err := runner(ctx, module.Dir, []string{"get", "-tool", toolRef}, stdout, stderr); err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeUnavailable, "bootstrap.Run", "declare Typology tool").
			With("module", module.Dir).
			With("tool", toolRef)
	}
	if err := runner(ctx, module.Dir, []string{"mod", "tidy"}, stdout, stderr); err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeUnavailable, "bootstrap.Run", "tidy consumer module").
			With("module", module.Dir)
	}
	if err := runner(ctx, module.Dir, []string{"tool", "typology", "version"}, stdout, stderr); err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeUnavailable, "bootstrap.Run", "verify Typology tool").
			With("module", module.Dir)
	}
	return Result{Module: module, Version: version}, nil
}

func toolVersion(raw string) (string, error) {
	version := strings.TrimSpace(raw)
	if version == "" {
		version = DefaultToolVersion
	}
	if !strings.HasPrefix(version, "v") || strings.ContainsAny(version, " \t\r\n") {
		return "", terrors.New(terrors.CodeInvalid, "bootstrap.toolVersion", "version must be a tagged Go module version such as v0.0.5").
			With("version", version)
	}
	return version, nil
}

func selectModule(modules []gorepo.Module, selector string) (gorepo.Module, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		if len(modules) == 1 {
			return modules[0], nil
		}
		return gorepo.Module{}, terrors.New(terrors.CodeInvalid, "bootstrap.selectModule", "multiple Go modules found; pass --module").
			With("modules", moduleNames(modules))
	}
	normalized := filepath.ToSlash(strings.TrimPrefix(selector, "./"))
	for _, module := range modules {
		if normalized == filepath.ToSlash(module.Rel) ||
			normalized == filepath.ToSlash(module.Path) ||
			filepath.Clean(selector) == filepath.Clean(module.Dir) {
			return module, nil
		}
	}
	return gorepo.Module{}, terrors.New(terrors.CodeNotFound, "bootstrap.selectModule", "module selector did not match a workspace module").
		With("selector", selector).
		With("modules", moduleNames(modules))
}

func moduleNames(modules []gorepo.Module) string {
	names := make([]string, 0, len(modules))
	for _, module := range modules {
		names = append(names, fmt.Sprintf("%s (%s)", module.Rel, module.Path))
	}
	return strings.Join(names, ", ")
}

func runGoCommand(ctx context.Context, dir string, args []string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
