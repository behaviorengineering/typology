package sourceindex

import (
	"fmt"
	"sort"
	"strings"
)

// MechanicalGrouping is the deterministic seed derived from roles and edges.
// It is facts only: no product names and no catalog objectives.
type MechanicalGrouping struct {
	DeliveryPaths   []string              `yaml:"delivery_paths,omitempty" json:"delivery_paths,omitempty"`
	EntrypointPaths []string              `yaml:"entrypoint_paths,omitempty" json:"entrypoint_paths,omitempty"`
	LibraryByRole   map[string][]string   `yaml:"library_by_role,omitempty" json:"library_by_role,omitempty"`
	ProductSeeds    []MechanicalComponent `yaml:"product_seeds,omitempty" json:"product_seeds,omitempty"`
	Notes           []string              `yaml:"notes,omitempty" json:"notes,omitempty"`
}

// MechanicalComponent is one connected set of aggregator/adapter packages.
type MechanicalComponent struct {
	Paths []string `yaml:"paths" json:"paths"`
}

// BuildMechanicalGrouping derives deterministic groups from a role topology.
func BuildMechanicalGrouping(topo RoleTopology) MechanicalGrouping {
	byPath := map[string]RoleNode{}
	for _, n := range topo.Packages {
		byPath[normalizePath(n.Path)] = n
	}
	out := MechanicalGrouping{
		LibraryByRole: map[string][]string{},
	}
	out.DeliveryPaths = pathsForRoleTopo(topo, RoleServer)
	out.EntrypointPaths = pathsForRoleTopo(topo, RoleEntrypoint)
	for _, role := range []string{RoleDTO, RoleConfig, RoleExecRunner, RoleObservability} {
		paths := pathsForRoleTopo(topo, role)
		if len(paths) == 0 {
			continue
		}
		out.LibraryByRole[role] = paths
	}
	for _, paths := range connectedProductComponents(topo) {
		out.ProductSeeds = append(out.ProductSeeds, MechanicalComponent{Paths: paths})
	}
	out.Notes = mechanicalGroupingNotes(topo, byPath, out.ProductSeeds)
	return out
}

// FormatMechanicalGroupingMarkdown renders the seed for LLM cluster input.
func FormatMechanicalGroupingMarkdown(g MechanicalGrouping) string {
	if len(g.DeliveryPaths) == 0 && len(g.EntrypointPaths) == 0 &&
		len(g.LibraryByRole) == 0 && len(g.ProductSeeds) == 0 && len(g.Notes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Mechanical grouping seed\n\n")
	b.WriteString("This seed is deterministic from package_roles and role edges. The LLM may extend or argue against it, but must not silently override it.\n\n")

	writePaths := func(title string, paths []string) {
		if len(paths) == 0 {
			return
		}
		fmt.Fprintf(&b, "### %s\n\n", title)
		for _, p := range paths {
			fmt.Fprintf(&b, "- `%s`\n", p)
		}
		b.WriteByte('\n')
	}
	writePaths("Delivery surfaces", g.DeliveryPaths)
	writePaths("Entry points", g.EntrypointPaths)

	if len(g.LibraryByRole) > 0 {
		fmt.Fprintf(&b, "### Library candidates\n\n")
		roles := make([]string, 0, len(g.LibraryByRole))
		for role := range g.LibraryByRole {
			roles = append(roles, role)
		}
		sort.Strings(roles)
		for _, role := range roles {
			paths := append([]string{}, g.LibraryByRole[role]...)
			sort.Strings(paths)
			quoted := make([]string, 0, len(paths))
			for _, p := range paths {
				quoted = append(quoted, "`"+p+"`")
			}
			fmt.Fprintf(&b, "- `%s`: %s\n", role, strings.Join(quoted, ", "))
		}
		b.WriteByte('\n')
	}

	fmt.Fprintf(&b, "### Product slice seeds\n\n")
	if len(g.ProductSeeds) == 0 {
		b.WriteString("- _(none)_\n\n")
	} else {
		for i, seed := range g.ProductSeeds {
			paths := append([]string{}, seed.Paths...)
			sort.Strings(paths)
			quoted := make([]string, 0, len(paths))
			for _, p := range paths {
				quoted = append(quoted, "`"+p+"`")
			}
			fmt.Fprintf(&b, "- seed %d: %s\n", i+1, strings.Join(quoted, ", "))
		}
		b.WriteByte('\n')
	}

	fmt.Fprintf(&b, "### Notes\n\n")
	if len(g.Notes) == 0 {
		b.WriteString("- _(none)_\n\n")
	} else {
		for _, note := range g.Notes {
			fmt.Fprintf(&b, "- %s\n", note)
		}
		b.WriteByte('\n')
	}

	fmt.Fprintf(&b, "### Rationale\n\n")
	b.WriteString("The grouping uses only role labels, evidence, and role edges. Folder names are ignored.\n")
	return strings.TrimSpace(b.String())
}

func pathsForRoleTopo(topo RoleTopology, role string) []string {
	var paths []string
	for _, n := range topo.PackagesByRole(role) {
		paths = append(paths, normalizePath(n.Path))
	}
	sort.Strings(paths)
	return paths
}

func connectedProductComponents(topo RoleTopology) [][]string {
	allowed := map[string]struct{}{}
	for _, n := range topo.Packages {
		switch strings.TrimSpace(n.Role) {
		case RoleAggregator, RoleAdapter:
			allowed[normalizePath(n.Path)] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	adj := map[string]map[string]struct{}{}
	add := func(a, b string) {
		if a == b {
			return
		}
		if adj[a] == nil {
			adj[a] = map[string]struct{}{}
		}
		adj[a][b] = struct{}{}
	}
	for _, e := range topo.Edges {
		from := normalizePath(e.From)
		to := normalizePath(e.To)
		if _, ok := allowed[from]; !ok {
			continue
		}
		if _, ok := allowed[to]; !ok {
			continue
		}
		add(from, to)
		add(to, from)
	}
	visited := map[string]struct{}{}
	var comps [][]string
	for path := range allowed {
		if _, ok := visited[path]; ok {
			continue
		}
		stack := []string{path}
		visited[path] = struct{}{}
		var comp []string
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			comp = append(comp, cur)
			for nxt := range adj[cur] {
				if _, ok := visited[nxt]; ok {
					continue
				}
				visited[nxt] = struct{}{}
				stack = append(stack, nxt)
			}
		}
		sort.Strings(comp)
		comps = append(comps, comp)
	}
	sort.Slice(comps, func(i, j int) bool {
		if len(comps[i]) != len(comps[j]) {
			return len(comps[i]) > len(comps[j])
		}
		return strings.Join(comps[i], ",") < strings.Join(comps[j], ",")
	})
	return comps
}

func mechanicalGroupingNotes(topo RoleTopology, byPath map[string]RoleNode, seeds []MechanicalComponent) []string {
	var notes []string
	if len(pathsForRoleTopo(topo, RoleEntrypoint)) > 0 && len(pathsForRoleTopo(topo, RoleServer)) > 0 {
		notes = append(notes, "`entrypoint` and `server` are distinct delivery roles; do not merge them by path words.")
	}
	for _, role := range []string{RoleDTO, RoleConfig, RoleExecRunner, RoleObservability} {
		if len(pathsForRoleTopo(topo, role)) == 0 {
			continue
		}
		notes = append(notes, fmt.Sprintf("`%s` packages are technical libraries, not product slices.", role))
	}
	for _, p := range pathsForRoleTopo(topo, RoleUnknown) {
		notes = append(notes, fmt.Sprintf("`%s` remains `unknown`; keep it out of automatic grouping until inspect resolves it.", p))
	}
	if len(seeds) == 0 {
		notes = append(notes, "No deterministic aggregator/adapter component was connected enough to seed a product slice.")
		return notes
	}
	for _, seed := range seeds {
		if len(seed.Paths) != 1 {
			continue
		}
		p := seed.Paths[0]
		node := byPath[p]
		notes = append(notes, fmt.Sprintf("`%s` is a singleton %s component; let the LLM argue whether it belongs with a neighbour or stays separate.", p, node.Role))
	}
	return notes
}
