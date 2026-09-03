package remediate

import (
	"fmt"
	"strings"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/validate"
)

// Options configures the remediate report.
type Options struct {
	RepoRoot string
	Catalog  catalog.Typology
	SliceID  string
	Modules  []string
	Module   string
}

// Report is agent-facing guidance for one remediation pass.
type Report struct {
	SliceID    string          `json:"sliceId"`
	Violations []catalog.Issue `json:"violations"`
	Protocol   []string        `json:"protocol"`
}

// Run returns violations scoped to one slice plus the agent protocol steps.
func Run(opts Options) (Report, error) {
	sliceID := strings.TrimSpace(opts.SliceID)
	if sliceID == "" {
		return Report{}, terrors.New(terrors.CodeInvalid, "remediate.Run", "slice id required")
	}
	if _, ok := opts.Catalog.LookupSlice(sliceID); !ok {
		return Report{}, terrors.New(terrors.CodeNotFound, "remediate.Run", "unknown slice").
			With("slice", sliceID)
	}
	all := validate.Run(validate.Options{
		RepoRoot: opts.RepoRoot,
		Catalog:  opts.Catalog,
		SliceID:  sliceID,
		Modules:  opts.Modules,
		Module:   opts.Module,
	})
	var scoped []catalog.Issue
	for _, issue := range all {
		if issue.Slice == "" || issue.Slice == sliceID {
			scoped = append(scoped, issue)
		}
	}
	catalog.SortIssues(scoped)
	return Report{
		SliceID:    sliceID,
		Violations: scoped,
		Protocol: []string{
			fmt.Sprintf("Load slice %q from %s.", sliceID, catalog.DefaultCatalogRel),
			fmt.Sprintf("Fix only packages owned by slice %q.", sliceID),
			fmt.Sprintf("Run: typology validate %s", opts.RepoRoot),
			"Stop when validate passes for that slice; do not refactor other slices.",
		},
	}, nil
}
