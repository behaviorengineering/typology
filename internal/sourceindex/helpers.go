package sourceindex

import "strings"

// RoleServer is the canonical delivery role name.
const RoleServer = RoleHTTPSurface

// HasEvidence reports whether the node evidence contains the requested token.
func (n RoleNode) HasEvidence(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	for _, e := range n.Evidence {
		if strings.TrimSpace(e) == token {
			return true
		}
	}
	return false
}

// Package looks up a package node by normalized path.
func (t RoleTopology) Package(path string) (RoleNode, bool) {
	want := normalizePath(path)
	for _, n := range t.Packages {
		if normalizePath(n.Path) == want {
			return n, true
		}
	}
	return RoleNode{}, false
}

// PackagesByRole returns all packages with the requested role.
func (t RoleTopology) PackagesByRole(role string) []RoleNode {
	role = strings.TrimSpace(role)
	if role == "" {
		return nil
	}
	var out []RoleNode
	for _, n := range t.Packages {
		if strings.TrimSpace(n.Role) == role {
			out = append(out, n)
		}
	}
	return out
}

// HasEdge reports whether an edge with the requested endpoints and kind exists.
func (t RoleTopology) HasEdge(fromPath, toPath, kind string) bool {
	fromPath = normalizePath(fromPath)
	toPath = normalizePath(toPath)
	kind = strings.TrimSpace(kind)
	for _, e := range t.Edges {
		if normalizePath(e.From) == fromPath && normalizePath(e.To) == toPath && strings.TrimSpace(e.Kind) == kind {
			return true
		}
	}
	return false
}
