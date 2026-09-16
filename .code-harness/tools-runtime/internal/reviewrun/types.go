package reviewrun

import "context"

const SchemaVersion = 180

type Intent struct {
	Mode   string `json:"mode"`
	Target string `json:"target"`
}

type Node struct {
	Path      string `json:"path"`
	Symbol    string `json:"symbol"`
	Role      string `json:"role"`
	Workspace string `json:"workspace"`
}

type Chain struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Nodes      []Node   `json:"nodes"`
	Unresolved []string `json:"unresolved"`
}

type ReadRef struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
}

type Evidence struct {
	Ref   ReadRef `json:"ref"`
	Quote string  `json:"quote"`
}

type Finding struct {
	ID                 string     `json:"id"`
	Severity           string     `json:"severity"`
	Problem            string     `json:"problem"`
	Impact             string     `json:"impact"`
	Recommendation     string     `json:"recommendation"`
	Verification       string     `json:"verification"`
	Evidence           []Evidence `json:"evidence"`
	IntroducedByChange *bool      `json:"introducedByChange"`
}

type FinishRequest struct {
	RunID        string    `json:"runId"`
	Reads        []ReadRef `json:"reads"`
	Findings     []Finding `json:"findings"`
	PendingRisks []string  `json:"pendingRisks"`
	Gaps         []string  `json:"gaps"`
}

type Options struct {
	RunID             string   `json:"runId"`
	Hash              string   `json:"optionsHash"`
	Chains            []Chain  `json:"chains"`
	DiscoveryComplete bool     `json:"discoveryComplete"`
	SelectionRequired bool     `json:"selectionRequired"`
	Gaps              []string `json:"gaps"`
	ReportPath        string   `json:"reportPath"`
}

type SelectionRequest struct {
	RunID       string   `json:"runId"`
	OptionsHash string   `json:"optionsHash"`
	IDs         []string `json:"selectionIds"`
}

type HostTurn struct {
	SessionID string `json:"sessionId"`
	MessageID string `json:"messageId"`
}

type Outcome struct {
	RunID            string `json:"runId"`
	Execution        string `json:"execution"`
	ReviewConclusion string `json:"reviewConclusion"`
	Coverage         string `json:"coverage"`
	ReportPath       string `json:"reportPath"`
	ReportSHA256     string `json:"reportSha256"`
}

// Prepare and Select are implemented by Task 2. The declarations below document the
// shared interface without pretending those behaviors are available in Task 1.
type PrepareFunc func(context.Context, string, string, Intent) (Options, error)
type SelectFunc func(context.Context, string, SelectionRequest, HostTurn) (Outcome, error)
