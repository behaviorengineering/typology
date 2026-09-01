// Package catalog holds the Typology domain model.
package catalog

// Gate is how an action or job may proceed.
type Gate string

const (
	GateAuto  Gate = "auto"
	GateTest  Gate = "test"
	GateHuman Gate = "human"
)

// Layer is domain vs interaction.
type Layer string

const (
	LayerDomain      Layer = "domain"
	LayerInteraction Layer = "interaction"
)

// InteractionKind tags interaction-layer components.
type InteractionKind string

const (
	InteractionUI  InteractionKind = "ui"
	InteractionCLI InteractionKind = "cli"
	InteractionAPI InteractionKind = "api"
)

// BindingRule is a component import rule.
type BindingRule string

const (
	BindingMust    BindingRule = "must"
	BindingMustNot BindingRule = "must_not"
	BindingReads   BindingRule = "reads"
)

// SliceBindingKind is coupling between slices.
type SliceBindingKind string

const (
	SliceConsumes SliceBindingKind = "consumes"
	SliceReads    SliceBindingKind = "reads"
)

// DocPageKind is a develop doc page type.
type DocPageKind string

const (
	DocOverview     DocPageKind = "overview"
	DocComponents   DocPageKind = "components"
	DocContracts    DocPageKind = "contracts"
	DocCLI          DocPageKind = "cli"
	DocPresentation DocPageKind = "presentation"
	DocPipelines    DocPageKind = "pipelines"
)

// DefaultDocPageKinds is the default DocPage order for a slice cluster.
var DefaultDocPageKinds = []DocPageKind{
	DocOverview,
	DocComponents,
	DocContracts,
	DocCLI,
	DocPresentation,
	DocPipelines,
}

// Typology is the whole architecture map.
type Typology struct {
	ID                string             `json:"id" yaml:"id"`
	Slices            []Slice            `json:"slices" yaml:"slices"`
	SliceBindings     []SliceBinding     `json:"sliceBindings,omitempty" yaml:"sliceBindings,omitempty"`
	ComponentBindings []ComponentBinding `json:"componentBindings,omitempty" yaml:"componentBindings,omitempty"`
}

// Slice is one bounded context.
type Slice struct {
	ID          string       `json:"id" yaml:"id"`
	Route       string       `json:"route,omitempty" yaml:"route,omitempty"`
	Objective   string       `json:"objective,omitempty" yaml:"objective,omitempty"`
	Must        []string     `json:"must,omitempty" yaml:"must,omitempty"`
	MustNot     []string     `json:"mustNot,omitempty" yaml:"mustNot,omitempty"`
	Success     string       `json:"success,omitempty" yaml:"success,omitempty"`
	Owns        []Component  `json:"owns,omitempty" yaml:"owns,omitempty"`
	OpRuns      []OpRun      `json:"opRuns,omitempty" yaml:"opRuns,omitempty"`
	Subprograms []Subprogram `json:"subprograms,omitempty" yaml:"subprograms,omitempty"`
	Actuators   []Actuator   `json:"actuators,omitempty" yaml:"actuators,omitempty"`
	Docs        DocCluster   `json:"docs,omitempty" yaml:"docs,omitempty"`
}

// Component is a package (or equivalent) inside a slice.
type Component struct {
	ID    string          `json:"id" yaml:"id"`
	Path  string          `json:"path" yaml:"path"`
	Layer Layer           `json:"layer" yaml:"layer"`
	Kind  InteractionKind `json:"kind,omitempty" yaml:"kind,omitempty"`
	Ops   bool            `json:"ops,omitempty" yaml:"ops,omitempty"`
}

// OpRun is one gated operator invocation, owned by a component. It may be
// a CLI, HTTP action, human gate, signal, or a later scheduled trigger.
// Optional Runs names the subprogram this invocation executes. Optional
// Actuates names the actuator this run may fire. At most one of Runs or
// Actuates.
type OpRun struct {
	ID             string `json:"id" yaml:"id"`
	OwnerComponent string `json:"ownerComponent" yaml:"ownerComponent"`
	Gate           Gate   `json:"gate,omitempty" yaml:"gate,omitempty"`
	CLI            string `json:"cli,omitempty" yaml:"cli,omitempty"`
	Runs           string `json:"runs,omitempty" yaml:"runs,omitempty"`
	Actuates       string `json:"actuates,omitempty" yaml:"actuates,omitempty"`
}

// Subprogram is a standing capability inside a slice (Input, Output, Store,
// Gate). An OpRun is one invocation of it. A Component is only the package
// it lives in. Consilium Contract().Subprograms map here. When Store is
// set, this subprogram is the origin for those paths or id kinds.
type Subprogram struct {
	ID             string   `json:"id" yaml:"id"`
	OwnerComponent string   `json:"ownerComponent" yaml:"ownerComponent"`
	Input          string   `json:"input,omitempty" yaml:"input,omitempty"`
	Output         string   `json:"output,omitempty" yaml:"output,omitempty"`
	Store          []string `json:"store,omitempty" yaml:"store,omitempty"`
	Gate           Gate     `json:"gate,omitempty" yaml:"gate,omitempty"`
}

// Actuator is a named, gated capability that fires when Signals arrive
// and emits an effect, usually past the instance edge (chatbot, email,
// counsel pack, public post). An OpRun is one invocation of it. A Component
// is only the package it lives in. Consilium Contract().Actuators map here.
type Actuator struct {
	ID             string   `json:"id" yaml:"id"`
	OwnerComponent string   `json:"ownerComponent" yaml:"ownerComponent"`
	Signals        []string `json:"signals,omitempty" yaml:"signals,omitempty"`
	Emits          []string `json:"emits,omitempty" yaml:"emits,omitempty"`
	Gate           Gate     `json:"gate,omitempty" yaml:"gate,omitempty"`
}

// SliceBinding couples two slices.
type SliceBinding struct {
	From string           `json:"from" yaml:"from"`
	To   string           `json:"to" yaml:"to"`
	Kind SliceBindingKind `json:"kind" yaml:"kind"`
}

// ComponentBinding couples two components.
type ComponentBinding struct {
	From string      `json:"from" yaml:"from"`
	To   string      `json:"to" yaml:"to"`
	Rule BindingRule `json:"rule" yaml:"rule"`
}

// DocCluster is the slice doc set.
type DocCluster struct {
	Pages []DocPage `json:"pages" yaml:"pages"`
}

// DocPage is one doc file.
type DocPage struct {
	Kind DocPageKind `json:"kind" yaml:"kind"`
	Path string      `json:"path" yaml:"path"`
}
