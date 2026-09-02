package catalog

import (
	"fmt"
	"sort"
	"strings"
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

// ComponentByID maps component id across all slices (owns + surface components).
func (t Typology) ComponentByID() map[string]Component {
	out := make(map[string]Component)
	for _, s := range t.Slices {
		for _, c := range s.AllComponents() {
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
		for _, c := range s.AllComponents() {
			if c.ID == componentID {
				return s.ID
			}
		}
	}
	return ""
}

// AllComponents returns domain owns plus every surface component.
func (s Slice) AllComponents() []Component {
	out := make([]Component, 0, len(s.Owns)+surfaceComponentCount(s.Surfaces))
	out = append(out, s.Owns...)
	for _, surf := range s.Surfaces {
		out = append(out, surf.Components...)
	}
	return out
}

func surfaceComponentCount(surfaces []Surface) int {
	n := 0
	for _, surf := range surfaces {
		n += len(surf.Components)
	}
	return n
}

// SurfaceByKind returns the first surface with the given kind, if any.
func (s Slice) SurfaceByKind(kind InteractionKind) (Surface, bool) {
	for _, surf := range s.Surfaces {
		if surf.Kind == kind {
			return surf, true
		}
	}
	return Surface{}, false
}

func docsDir(sliceID, docsRoot string) string {
	prefix := strings.TrimSpace(docsRoot)
	if prefix == "" {
		prefix = DefaultDocsRoot
	}
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return prefix + sliceID + "/"
}

func docPage(sliceID, docsRoot string, kind DocPageKind) DocPage {
	return DocPage{Kind: kind, Path: docsDir(sliceID, docsRoot) + string(kind) + ".md"}
}

// SliceReadmePath is the slice hub README under the docs root.
func SliceReadmePath(sliceID, docsRoot string) string {
	return docsDir(sliceID, docsRoot) + "README.md"
}

// SubprogramPagePath is the conventional extra leaf for a subprogram.
func SubprogramPagePath(sliceID, subprogramID, docsRoot string) string {
	return docsDir(sliceID, docsRoot) + "subprograms/" + subprogramID + ".md"
}

// ActuatorPagePath is the conventional extra leaf for an actuator.
func ActuatorPagePath(sliceID, actuatorID, docsRoot string) string {
	return docsDir(sliceID, docsRoot) + "actuators/" + actuatorID + ".md"
}

// DefaultDocCluster builds DocPages for a slice. Overview and components always
// appear. Surface kinds add contracts (api), cli, and presentation (ui).
// Pipelines is included when the slice lists opRuns. Callers MAY still list all
// DefaultDocPageKinds in YAML to opt into empty surface pages.
func DefaultDocCluster(s Slice, docsRoot string) DocCluster {
	id := strings.TrimSpace(s.ID)
	pages := []DocPage{
		docPage(id, docsRoot, DocOverview),
		docPage(id, docsRoot, DocComponents),
	}
	if _, ok := s.SurfaceByKind(InteractionAPI); ok {
		pages = append(pages, docPage(id, docsRoot, DocContracts))
	}
	if _, ok := s.SurfaceByKind(InteractionCLI); ok {
		pages = append(pages, docPage(id, docsRoot, DocCLI))
	}
	if _, ok := s.SurfaceByKind(InteractionUI); ok {
		pages = append(pages, docPage(id, docsRoot, DocPresentation))
	}
	if len(s.OpRuns) > 0 {
		pages = append(pages, docPage(id, docsRoot, DocPipelines))
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
			if c.Layer == LayerInteraction {
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("component %q: interaction packages belong on surfaces, not owns", c.ID),
				})
			}
			if c.Layer != LayerDomain {
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("component %q: owns[] requires layer domain", c.ID),
				})
			}
		}
		issues = append(issues, s.validateSurfaces(seenComp)...)
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

func (s Slice) validateSurfaces(seenComp map[string]string) []Issue {
	var issues []Issue
	seenSurface := map[string]struct{}{}
	seenKind := map[InteractionKind]struct{}{}
	for _, surf := range s.Surfaces {
		if surf.ID == "" {
			issues = append(issues, Issue{Slice: s.ID, Message: "surface with empty id"})
			continue
		}
		if _, ok := seenSurface[surf.ID]; ok {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("duplicate surface id %q", surf.ID),
			})
		}
		seenSurface[surf.ID] = struct{}{}
		if s.hasSubprogram(surf.ID) {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("surface %q: id already used by a subprogram", surf.ID),
			})
		}
		if s.hasActuator(surf.ID) {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("surface %q: id already used by an actuator", surf.ID),
			})
		}
		if surf.Kind == "" {
			issues = append(issues, Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("surface %q: missing kind ui|cli|api", surf.ID),
			})
		} else {
			switch surf.Kind {
			case InteractionUI, InteractionCLI, InteractionAPI:
				if _, ok := seenKind[surf.Kind]; ok {
					issues = append(issues, Issue{
						Slice:   s.ID,
						Message: fmt.Sprintf("surface %q: duplicate kind %q on slice", surf.ID, surf.Kind),
					})
				}
				seenKind[surf.Kind] = struct{}{}
			default:
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("surface %q: kind must be ui|cli|api", surf.ID),
				})
			}
		}
		for _, c := range surf.Components {
			if c.ID == "" {
				issues = append(issues, Issue{Slice: s.ID, Message: fmt.Sprintf("surface %q: component with empty id", surf.ID)})
				continue
			}
			if owner, ok := seenComp[c.ID]; ok {
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("duplicate component id %q (also on slice %q)", c.ID, owner),
				})
			}
			seenComp[c.ID] = s.ID
			if c.Layer == LayerInteraction || c.Kind != "" {
				issues = append(issues, Issue{
					Slice:   s.ID,
					Message: fmt.Sprintf("surface component %q: layer/kind belong on the surface, not nested components", c.ID),
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
				Message: fmt.Sprintf("subprogram %q: ownerComponent %q not in slice owns or surfaces", sp.ID, sp.OwnerComponent),
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
				Message: fmt.Sprintf("actuator %q: ownerComponent %q not in slice owns or surfaces", a.ID, a.OwnerComponent),
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
				Message: fmt.Sprintf("opRun %q: ownerComponent %q not in slice owns or surfaces", r.ID, r.OwnerComponent),
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
	for _, c := range s.AllComponents() {
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

// BuildSurfaces groups interaction components into one surface per kind (ui, cli, api).
func BuildSurfaces(sliceID string, byKind map[InteractionKind][]Component) []Surface {
	order := []InteractionKind{InteractionUI, InteractionCLI, InteractionAPI}
	out := make([]Surface, 0, len(byKind))
	for _, kind := range order {
		comps := byKind[kind]
		if len(comps) == 0 {
			continue
		}
		out = append(out, Surface{
			ID:         sliceID + "-" + string(kind),
			Kind:       kind,
			Components: comps,
		})
	}
	return out
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
