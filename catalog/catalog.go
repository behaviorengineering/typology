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

// DefaultDocCluster builds the six-page develop pack for a slice.
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
		for _, j := range s.Jobs {
			if j.ID == "" {
				issues = append(issues, Issue{Slice: s.ID, Message: "job with empty id"})
				continue
			}
			if j.OwnerComponent == "" {
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("job %q: missing ownerComponent", j.ID),
				})
				continue
			}
			if !s.hasComponent(j.OwnerComponent) {
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("job %q: ownerComponent %q not in slice owns", j.ID, j.OwnerComponent),
				})
			}
		}
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

func (s Slice) hasComponent(id string) bool {
	for _, c := range s.Owns {
		if c.ID == id {
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
