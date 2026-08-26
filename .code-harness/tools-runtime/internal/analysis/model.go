package analysis

type Intent struct {
	Mode   string `json:"mode"`
	Target string `json:"target,omitempty"`
}

type ChangedFile struct {
	Path string `json:"path"`
	Role string `json:"role"`
}

type AffectedController struct {
	Controller    string   `json:"controller"`
	Endpoints     []string `json:"endpoints"`
	ImpactType    string   `json:"impactType"`
	SourceSymbols []string `json:"sourceSymbols"`
}

type CallChain struct {
	EntryPoint string   `json:"entryPoint"`
	Chain      []string `json:"chain"`
}

type SymbolLocation struct {
	Workspace string `json:"workspace,omitempty"`
	Symbol    string `json:"symbol"`
	Path      string `json:"path"`
	Role      string `json:"role"`
	Source    string `json:"source"`
	From      string `json:"from,omitempty"`
}

type ResourceRelation struct {
	Path       string `json:"path"`
	Role       string `json:"role"`
	Resource   string `json:"resource"`
	FromSymbol string `json:"fromSymbol"`
	FromKind   string `json:"fromKind"`
	Source     string `json:"source"`
	Evidence   string `json:"evidence"`
}

type UnresolvedSymbol struct {
	Symbol string `json:"symbol"`
	From   string `json:"from"`
	Reason string `json:"reason"`
}

type ReviewCoverage struct {
	Status            string             `json:"status"`
	ReviewedFiles     []ChangedFile      `json:"reviewedFiles"`
	UnresolvedSymbols []UnresolvedSymbol `json:"unresolvedSymbols"`
}

type ChangeAnalysis struct {
	ChangedFiles         []ChangedFile        `json:"changedFiles"`
	AffectedControllers  []AffectedController `json:"affectedControllers"`
	CallChains           []CallChain          `json:"callChains"`
	SymbolLocations      []SymbolLocation     `json:"symbolLocations"`
	ResourceRelations    []ResourceRelation   `json:"resourceRelations"`
	ExternalDependencies []string             `json:"externalDependencies"`
	ReviewCoverage       ReviewCoverage       `json:"reviewCoverage"`
}

type EntrypointDisposition string

const (
	DispositionConfirmed EntrypointDisposition = "CONFIRMED"
	DispositionPartial   EntrypointDisposition = "PARTIAL"
	DispositionRemoved   EntrypointDisposition = "REMOVED"
)

type ExpectedEntrypoint struct {
	Symbol      string                `json:"symbol"`
	Path        string                `json:"path"`
	Disposition EntrypointDisposition `json:"disposition,omitempty"`
	Limitation  string                `json:"limitation,omitempty"`
}

type EntrypointInventory struct {
	RunID               string               `json:"runId"`
	Status              string               `json:"status"`
	ExpectedEntrypoints []ExpectedEntrypoint `json:"expectedEntryPoints"`
	ChangeSetSHA256     string               `json:"changeSetSha256"`
	Intent              *Intent              `json:"intent,omitempty"`
}
