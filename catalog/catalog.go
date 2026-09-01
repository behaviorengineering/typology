package catalog

import (
	"fmt"
	"sort"
)

// LookupSlice returns a slice by id.
func (t Typology) LookupSlice(id string) (Slice, bool) {
	for _, s := range t.Slices {
		if s.ID == id {
			return s, true
		}
	}
	return Slice{}, false
}

// ComponentByID maps component id across all slices.
func (t Typology) ComponentByID() map[string]Component {
	out := make(map[string]Component)
	for _, s := range t.Slices {
		for _, c := range s.Owns {
			if c.ID != "" {
				out[c.ID] = c
			}
		}
	}
	return out
}

// SliceForComponent returns the slice id owning a component id.
func (t Typology) SliceForComponent(componentID string) string {
	for _, s := range t.Slices {
		for _, c := range s.Owns {
			if c.ID == componentID {
				return s.ID
			}
		}
	}
	return ""
}

// DefaultDocCluster builds the six-page doc pack for a slice.
func DefaultDocCluster(sliceID, docsRoot string) DocCluster {
	prefix := docsRoot
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	base := prefix + sliceID + "/"
	pages := make([]DocPage, 0, len(DefaultDocPageKinds))
	for _, kind := range DefaultDocPageKinds {
		pages = append(pages, DocPage{
			Kind: kind,
			Path: base + string(kind) + ".md",
		})
	}
	return DocCluster{Pages: pages}
}

// ValidateStructure checks ids and cross references without touching the repo.
func (t Typology) ValidateStructure() []Issue {
	var issues []Issue
	seenSlice := map[string]struct{}{}
	seenComp := map[string]string{}
	for _, s := range t.Slices {
		if s.ID == "" {
			issues = append(issues, Issue{Message: "slice with empty id"})
			continue
		}
		if _, ok := seenSlice[s.ID]; ok {
			issues = append(issues, Issue{Slice: s.ID, Message: "duplicate slice id"})
		}
		seenSlice[s.ID] = struct{}{}
		for _, c := range s.Owns {
			if c.ID == "" {
				issues = append(issues, Issue{Slice: s.ID, Message: "component with empty id"})
				continue
			}
			if owner, ok := seenComp[c.ID]; ok {
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("duplicate component id %q (also on slice %q)", c.ID, owner),
				})
			}
			seenComp[c.ID] = s.ID
			if c.Layer == LayerInteraction && c.Kind == "" {
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("component %q: interaction layer requires kind ui|cli|api", c.ID),
				})
			}
		}
		issues = append(issues, s.validateSubprograms()...)
		issues = append(issues, s.validateActuators()...)
		issues = append(issues, s.validateOpRuns()...)
	}
	for _, b := range t.SliceBindings {
		if _, ok := seenSlice[b.From]; !ok {
			issues = append(issues, Issue{Message: fmt.Sprintf("SliceBinding from unknown slice %q", b.From)})
		}
		if _, ok := seenSlice[b.To]; !ok {
			issues = append(issues, Issue{Message: fmt.Sprintf("SliceBinding to unknown slice %q", b.To)})
		}
	}
	for _, b := range t.ComponentBindings {
		fromSlice := t.SliceForComponent(b.From)
		toSlice := t.SliceForComponent(b.To)
		if fromSlice == "" {
			issues = append(issues, Issue{Message: fmt.Sprintf("ComponentBinding from unknown component %q", b.From)})
			continue
		}
		if toSlice == "" {
			issues = append(issues, Issue{Message: fmt.Sprintf("ComponentBinding to unknown component %q", b.To)})
			continue
		}
		if fromSlice != toSlice {
			hasSliceBinding := false
			for _, sb := range t.SliceBindings {
				if (sb.From == fromSlice && sb.To == toSlice) || (sb.From == toSlice && sb.To == fromSlice) {
					hasSliceBinding = true
					break
				}
			}
			if !hasSliceBinding {
				issues = append(issues, Issue{
					Slice:   fromSlice,
					Message: fmt.Sprintf("ComponentBinding %q -> %q crosses slices without SliceBinding", b.From, b.To),
				})
			}
		}
	}
	return issues
}

func (s Slice) validateSubprograms() []Issue {
	var issues []Issue
	seen := map[string]struct{}{}
	for _, sp := range s.Subprograms {
		if sp.ID == "" {
			issues = append(issues, Issue{Slice: s.ID, Message: "subprogram with empty id"})
			continue
		}
		if _, ok := seen[sp.ID]; ok {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("duplicate subprogram id %q", sp.ID),
			})
		}
		seen[sp.ID] = struct{}{}
		if sp.OwnerComponent == "" {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("subprogram %q: missing ownerComponent", sp.ID),
			})
			continue
		}
		if !s.hasComponent(sp.OwnerComponent) {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("subprogram %q: ownerComponent %q not in slice owns", sp.ID, sp.OwnerComponent),
			})
		}
	}
	return issues
}

func (s Slice) validateActuators() []Issue {
	var issues []Issue
	seen := map[string]struct{}{}
	for _, a := range s.Actuators {
		if a.ID == "" {
			issues = append(issues, Issue{Slice: s.ID, Message: "actuator with empty id"})
			continue
		}
		if _, ok := seen[a.ID]; ok {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("duplicate actuator id %q", a.ID),
			})
		}
		seen[a.ID] = struct{}{}
		if s.hasSubprogram(a.ID) {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("actuator %q: id already used by a subprogram", a.ID),
			})
		}
		if a.OwnerComponent == "" {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("actuator %q: missing ownerComponent", a.ID),
			})
		} else if !s.hasComponent(a.OwnerComponent) {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("actuator %q: ownerComponent %q not in slice owns", a.ID, a.OwnerComponent),
			})
		}
		if len(a.Signals) == 0 {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("actuator %q: missing signals", a.ID),
			})
		}
		if len(a.Emits) == 0 {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("actuator %q: missing emits", a.ID),
			})
		}
	}
	return issues
}

func (s Slice) validateOpRuns() []Issue {
	var issues []Issue
	for _, r := range s.OpRuns {
		if r.ID == "" {
			issues = append(issues, Issue{Slice: s.ID, Message: "opRun with empty id"})
			continue
		}
		if r.OwnerComponent == "" {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("opRun %q: missing ownerComponent", r.ID),
			})
			continue
		}
		if !s.hasComponent(r.OwnerComponent) {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("opRun %q: ownerComponent %q not in slice owns", r.ID, r.OwnerComponent),
			})
		}
		if r.Runs != "" && r.Actuates != "" {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("opRun %q: runs and actuates are mutually exclusive", r.ID),
			})
		}
		if r.Runs != "" && !s.hasSubprogram(r.Runs) {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("opRun %q: runs %q is not a subprogram on this slice", r.ID, r.Runs),
			})
		}
		if r.Actuates != "" && !s.hasActuator(r.Actuates) {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("opRun %q: actuates %q is not an actuator on this slice", r.ID, r.Actuates),
			})
		}
	}
	return issues
}

func (s Slice) hasComponent(id string) bool {
	for _, c := range s.Owns {
		if c.ID == id {
			return true
		}
	}
	return false
}

func (s Slice) hasSubprogram(id string) bool {
	for _, sp := range s.Subprograms {
		if sp.ID == id {
			return true
		}
	}
	return false
}

func (s Slice) hasActuator(id string) bool {
	for _, a := range s.Actuators {
		if a.ID == id {
			return true
		}
	}
	return false
}

// Issue is one validate finding.
type Issue struct {
	Slice   string `json:"slice,omitempty"`
	Message string `json:"message"`
}

// SortIssues orders findings for stable output.
func SortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Slice != issues[j].Slice {
			return issues[i].Slice < issues[j].Slice
		}
		return issues[i].Message < issues[j].Message
	})
}
