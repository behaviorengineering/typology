package sourceindex

import (
	"fmt"
	"sort"
	"strings"
)

// MechanicalGrouping is the deterministic seed derived from door walks on roles and edges.
// It is facts only: no product names and no catalog objectives.
type MechanicalGrouping struct {
	DeliveryPaths   []string              `yaml:"delivery_paths,omitempty" json:"delivery_paths,omitempty"`
	EntrypointPaths []string              `yaml:"entrypoint_paths,omitempty" json:"entrypoint_paths,omitempty"`
	DoorWalks       []DoorWalk            `yaml:"door_walks,omitempty" json:"door_walks,omitempty"`
	SharedPaths     []string              `yaml:"shared_paths,omitempty" json:"shared_paths,omitempty"`
	UnreachedPaths  []string              `yaml:"unreached_paths,omitempty" json:"unreached_paths,omitempty"`
	LibraryByRole   map[string][]string   `yaml:"library_by_role,omitempty" json:"library_by_role,omitempty"`
	ProductSeeds    []MechanicalComponent `yaml:"product_seeds,omitempty" json:"product_seeds,omitempty"`
	Notes           []string              `yaml:"notes,omitempty" json:"notes,omitempty"`
}

// DoorWalk is one entrypoint or server reachability result.
type DoorWalk struct {
	DoorPath        string   `yaml:"door_path" json:"door_path"`
	DoorRole        string   `yaml:"door_role" json:"door_role"`
	PrivatePaths    []string `yaml:"private_paths,omitempty" json:"private_paths,omitempty"`
	CrossDoorWiring []string `yaml:"cross_door_wiring,omitempty" json:"cross_door_wiring,omitempty"`
}

// MechanicalComponent is one connected set of aggregator/adapter packages.
type MechanicalComponent struct {
	Paths []string `yaml:"paths" json:"paths"`
}

// BuildMechanicalGrouping derives deterministic groups via door-started walks.
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

	doors := doorSet(topo)
	adj := outboundAdj(topo)

	reachedBy := map[string]map[string]struct{}{} // package -> doors that reach it
	crossByDoor := map[string][]string{}
	reachedFromDoor := map[string]map[string]struct{}{}

	doorList := make([]string, 0, len(doors))
	for d := range doors {
		doorList = append(doorList, d)
	}
	sort.Strings(doorList)

	for _, door := range doorList {
		reached, cross := walkFromDoor(door, doors, adj)
		reachedFromDoor[door] = reached
		crossByDoor[door] = cross
		for p := range reached {
			if reachedBy[p] == nil {
				reachedBy[p] = map[string]struct{}{}
			}
			reachedBy[p][door] = struct{}{}
		}
	}

	sharedSet := map[string]struct{}{}
	for p, ds := range reachedBy {
		if _, isDoor := doors[p]; isDoor {
			continue
		}
		if len(ds) >= 2 {
			sharedSet[p] = struct{}{}
			out.SharedPaths = append(out.SharedPaths, p)
		}
	}
	sort.Strings(out.SharedPaths)

	privateByDoor := map[string][]string{}
	for _, door := range doorList {
		var private []string
		for p := range reachedFromDoor[door] {
			if p == door {
				continue
			}
			if _, isDoor := doors[p]; isDoor {
				continue
			}
			if _, shared := sharedSet[p]; shared {
				continue
			}
			private = append(private, p)
		}
		sort.Strings(private)
		privateByDoor[door] = private
		node := byPath[door]
		out.DoorWalks = append(out.DoorWalks, DoorWalk{
			DoorPath:        door,
			DoorRole:        strings.TrimSpace(node.Role),
			PrivatePaths:    private,
			CrossDoorWiring: crossByDoor[door],
		})
	}

	for _, n := range topo.Packages {
		p := normalizePath(n.Path)
		if _, isDoor := doors[p]; isDoor {
			continue
		}
		if _, ok := reachedBy[p]; ok {
			continue
		}
		out.UnreachedPaths = append(out.UnreachedPaths, p)
	}
	sort.Strings(out.UnreachedPaths)

	for _, role := range []string{RoleDTO, RoleConfig, RoleExecRunner, RoleObservability} {
		paths := pathsForRoleTopo(topo, role)
		if len(paths) == 0 {
			continue
		}
		out.LibraryByRole[role] = paths
	}

	privateAllowed := map[string]struct{}{}
	for _, paths := range privateByDoor {
		for _, p := range paths {
			privateAllowed[p] = struct{}{}
		}
	}
	for _, paths := range connectedProductComponentsRestricted(topo, privateAllowed) {
		out.ProductSeeds = append(out.ProductSeeds, MechanicalComponent{Paths: paths})
	}

	out.Notes = mechanicalGroupingNotes(topo, byPath, out, doors, sharedSet)
	return out
}

// FormatMechanicalGroupingMarkdown renders the seed for LLM cluster input.
func FormatMechanicalGroupingMarkdown(g MechanicalGrouping) string {
	if len(g.DeliveryPaths) == 0 && len(g.EntrypointPaths) == 0 &&
		len(g.DoorWalks) == 0 && len(g.SharedPaths) == 0 && len(g.UnreachedPaths) == 0 &&
		len(g.LibraryByRole) == 0 && len(g.ProductSeeds) == 0 && len(g.Notes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Mechanical grouping seed\n\n")
	b.WriteString("This seed is deterministic from package_roles door walks. The LLM may extend or argue against it, but must not silently override door-private vs shared facts.\n\n")

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

	fmt.Fprintf(&b, "### Door walks\n\n")
	if len(g.DoorWalks) == 0 {
		b.WriteString("- _(none)_\n\n")
	} else {
		for _, w := range g.DoorWalks {
			fmt.Fprintf(&b, "- `%s` (`%s`)\n", w.DoorPath, w.DoorRole)
			if len(w.CrossDoorWiring) > 0 {
				for _, note := range w.CrossDoorWiring {
					fmt.Fprintf(&b, "  - cross-door: %s\n", note)
				}
			}
		}
		b.WriteByte('\n')
	}

	fmt.Fprintf(&b, "### Door-private packages\n\n")
	if len(g.DoorWalks) == 0 {
		b.WriteString("- _(none)_\n\n")
	} else {
		any := false
		for _, w := range g.DoorWalks {
			if len(w.PrivatePaths) == 0 {
				continue
			}
			any = true
			quoted := make([]string, 0, len(w.PrivatePaths))
			for _, p := range w.PrivatePaths {
				quoted = append(quoted, "`"+p+"`")
			}
			fmt.Fprintf(&b, "- `%s`: %s\n", w.DoorPath, strings.Join(quoted, ", "))
		}
		if !any {
			b.WriteString("- _(none)_\n")
		}
		b.WriteByte('\n')
	}

	writePaths("Shared across doors", g.SharedPaths)
	writePaths("Unreached", g.UnreachedPaths)

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
	b.WriteString("Walks start at entrypoint and server doors, follow From→To edges, and stop expanding through other doors. Shared means reached from two or more doors. Folder names are ignored.\n")
	return strings.TrimSpace(b.String())
}

func doorSet(topo RoleTopology) map[string]struct{} {
	out := map[string]struct{}{}
	for _, n := range topo.Packages {
		switch strings.TrimSpace(n.Role) {
		case RoleEntrypoint, RoleServer:
			out[normalizePath(n.Path)] = struct{}{}
		}
	}
	return out
}

func outboundAdj(topo RoleTopology) map[string][]roleHop {
	adj := map[string][]roleHop{}
	for _, e := range topo.Edges {
		from := normalizePath(e.From)
		to := normalizePath(e.To)
		if from == "" || to == "" || from == to {
			continue
		}
		adj[from] = append(adj[from], roleHop{To: to, Kind: strings.TrimSpace(e.Kind)})
	}
	return adj
}

type roleHop struct {
	To   string
	Kind string
}

func walkFromDoor(door string, doors map[string]struct{}, adj map[string][]roleHop) (reached map[string]struct{}, cross []string) {
	reached = map[string]struct{}{door: {}}
	queue := []string{door}
	crossSeen := map[string]struct{}{}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, hop := range adj[cur] {
			if _, isDoor := doors[hop.To]; isDoor && hop.To != door {
				kind := hop.Kind
				if kind == "" {
					kind = "edge"
				}
				note := fmt.Sprintf("`%s` -[%s]-> `%s`", cur, kind, hop.To)
				if _, ok := crossSeen[note]; !ok {
					crossSeen[note] = struct{}{}
					cross = append(cross, note)
				}
				continue
			}
			if _, ok := reached[hop.To]; ok {
				continue
			}
			reached[hop.To] = struct{}{}
			if _, isDoor := doors[hop.To]; isDoor {
				continue
			}
			queue = append(queue, hop.To)
		}
	}
	sort.Strings(cross)
	return reached, cross
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
	return connectedComponents(allowed, topo)
}

func connectedProductComponentsRestricted(topo RoleTopology, allowed map[string]struct{}) [][]string {
	filtered := map[string]struct{}{}
	for _, n := range topo.Packages {
		p := normalizePath(n.Path)
		if _, ok := allowed[p]; !ok {
			continue
		}
		switch strings.TrimSpace(n.Role) {
		case RoleAggregator, RoleAdapter:
			filtered[p] = struct{}{}
		}
	}
	return connectedComponents(filtered, topo)
}

func connectedComponents(allowed map[string]struct{}, topo RoleTopology) [][]string {
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

func mechanicalGroupingNotes(topo RoleTopology, byPath map[string]RoleNode, g MechanicalGrouping, doors, sharedSet map[string]struct{}) []string {
	var notes []string
	if len(g.EntrypointPaths) > 0 && len(g.DeliveryPaths) > 0 {
		notes = append(notes, "`entrypoint` and `server` are distinct delivery doors; do not merge them.")
	}
	if len(doors) == 0 {
		notes = append(notes, "No entrypoint or server doors found; door walks are empty.")
	}
	for _, role := range []string{RoleDTO, RoleConfig, RoleExecRunner, RoleObservability} {
		if len(pathsForRoleTopo(topo, role)) == 0 {
			continue
		}
		notes = append(notes, fmt.Sprintf("`%s` packages are technical libraries, not product slices.", role))
	}
	for _, p := range g.SharedPaths {
		notes = append(notes, fmt.Sprintf("`%s` is shared across doors; keep it library-leaning unless evidence says otherwise.", p))
	}
	for _, p := range g.UnreachedPaths {
		node := byPath[p]
		role := strings.TrimSpace(node.Role)
		if role == "" {
			role = RoleUnknown
		}
		notes = append(notes, fmt.Sprintf("`%s` (`%s`) was not reached from any door; do not auto-own it.", p, role))
	}
	if len(g.ProductSeeds) == 0 {
		notes = append(notes, "No door-private aggregator/adapter component was connected enough to seed a product slice.")
		return notes
	}
	for _, seed := range g.ProductSeeds {
		if len(seed.Paths) != 1 {
			continue
		}
		p := seed.Paths[0]
		node := byPath[p]
		notes = append(notes, fmt.Sprintf("`%s` is a singleton %s component; let the LLM argue whether it belongs with a neighbour or stays separate.", p, node.Role))
	}
	return notes
}
