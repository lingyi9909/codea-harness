# Codea Harness 1.5.2 — Workspace Dependency Chain Navigation Design

## Goal

Allow `harness chain discover <Controller>` to traverse a VERIFIED, explicitly configured direct Maven workspace dependency in a sibling directory, resolve superclass/template-method/override flow there, and return to the current project while preserving strict Change Set / Review / Write isolation.

## Baseline

- Base branch: `develop`
- Exact 1.5.1 baseline: `cd020fb319af95d28518b484593e2e0a0a03b7b5`
- 1.5.1 Chain Discover Bootstrap behavior is preserved.

## Global boundary

```text
Workspace Dependency = Navigation Scope
Workspace Dependency ≠ Change Set
Workspace Dependency ≠ Review Scope
Workspace Dependency ≠ Write Scope
```

Only explicit direct sibling Maven workspace dependencies are supported. No arbitrary `../**` scanning, JAR decompilation, Maven/Nexus/Central network resolution, cross-repository graph, dependency tests, findings, writes, or dependency run state.

## Task 1 — Workspace Dependency Contract

`harness.yaml` may optionally declare:

```yaml
workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
```

Requirements:
- optional; 1.5.1 config remains valid and config `version` stays unchanged;
- unique stable id;
- unique root;
- only direct sibling directory of current project;
- root exists and is a directory;
- cannot point to current project;
- only `READ_ONLY` mode;
- reject traversal, UNC/non-sibling paths, and symlink escape;
- Project Adapter never auto-scans siblings and only records explicit dependency configuration.

## Task 2 — Maven source identity verification

A configured sibling is not trusted merely because its directory name matches. Runtime must verify the current project declares a direct Maven dependency and the sibling `pom.xml` resolves to the configured `groupId` / `artifactId` and, when deterministically resolvable from local POM information, the same version.

Allowed local resolution sources:
- direct version;
- `${property}`;
- local parent POM;
- local dependencyManagement.

No network or downloaded parent resolution.

Machine states:

```text
VERIFIED
VERSION_UNRESOLVED
COORDINATE_MISMATCH
VERSION_MISMATCH
SOURCE_NOT_FOUND
```

Only `VERIFIED` becomes confirmed Workspace Source. Version unresolved or mismatch must stop workspace source use and produce explicit machine limitation facts.

## Task 3 — Cross-workspace inheritance navigation

Only deterministic Java cases are supported:
- single exact superclass;
- exact inherited method;
- unique dispatch;
- `execute()` / `super.execute()` into superclass;
- superclass internal method calls;
- template-method call to an abstract/overridable hook and unique dispatch back to the concrete current-project override.

Required limitations include:

```text
WORKSPACE_DEPENDENCY_NOT_CONFIGURED
WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND
WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH
WORKSPACE_DEPENDENCY_VERSION_UNRESOLVED
WORKSPACE_DEPENDENCY_VERSION_MISMATCH
SUPERCLASS_NOT_FOUND
INHERITED_METHOD_NOT_FOUND
AMBIGUOUS_INHERITED_METHOD
TEMPLATE_OVERRIDE_NOT_FOUND
AMBIGUOUS_TEMPLATE_DISPATCH
```

Ambiguity must be `PARTIAL`; never guess.

## Task 4 — ChangeAnalysis / Chain integration

Workspace evidence uses logical workspace identity rather than `../` paths:

```json
{
  "workspace": "company-framework",
  "symbol": "AbstractTemplate.execute",
  "path": "src/main/java/com/company/AbstractTemplate.java",
  "role": "Service",
  "source": "WORKSPACE_INHERITANCE"
}
```

`workspace=current` identifies current-repository evidence. Missing workspace on old Chain YAML means `current`, so no migration of existing `chains/**` is required.

Chain deterministic identity includes workspace + symbol + path + role + order.

Workspace dependency may contribute only to:
- Navigation Evidence;
- Call Chain Context;
- Chain Validation.

It may not contribute to:
- current Change Set;
- Review Coverage required files;
- Review Finding file targets;
- Apply/Fix/Test write scope.

## Task 5 — Real business regression

Use a real two-project fixture:

```text
workspace/
├── order-service/
└── company-framework/
```

`company-framework` contains `AbstractTemplate.execute()` calling `before()`, abstract `doExecute()`, and `after()`.

`order-service` contains Controller → Service → ServiceImpl extends `AbstractTemplate` → override `doExecute()` → Mapper → Mapper.xml.

Direct user intent:

```text
harness chain discover XxxController
```

must bootstrap ChangeAnalysis without pre-writing Chain, verify Maven workspace identity, traverse template inheritance, return to the current override, and emit one DISCOVERED Chain containing both `current` and `company-framework` workspaces while writing state only under the current project's `runs/**`.

Failure regressions:
- dependency version mismatch → `PARTIAL / WORKSPACE_DEPENDENCY_VERSION_MISMATCH`;
- dependency not configured → `PARTIAL / WORKSPACE_DEPENDENCY_NOT_CONFIGURED`;
- ambiguous override → `PARTIAL / AMBIGUOUS_TEMPLATE_DISPATCH`;
- source missing → `PARTIAL / WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND`.

## Task 6 — Review isolation

With a workspace Chain present, both FULL and TARGETED Review retain current-repository-only Change Set / coverage / scopedFiles / Finding / Fix / Apply behavior. Dependency source is read-only navigation context only.

## Task 7 — Release gate

Release version `1.5.2`.

Required gates:
- `go test ./...`;
- `go vet ./...`;
- Windows x64 build;
- real ast-grep navigation smoke;
- workspace dependency regression;
- template inheritance regression;
- Chain regression;
- Review isolation regression;
- real `1.5.1 → 1.5.2` live upgrade.

`workspaceDependencies` is optional; existing 1.5.1 `harness.yaml` must not be forcibly rewritten. Existing Project State protection remains for `harness.yaml`, `project.md`, `database.yaml`, `runs/**`, and `chains/**`.

Release artifacts:

```text
codea-harness-1.5.2-windows-x64-install.zip
codea-harness-1.5.2-windows-x64-upgrade.zip
```

## Acceptance gates

```text
G1  Workspace dependency is explicit allowlist only
G2  Maven artifact identity is verified
G3  Wrong/unresolved version is never used as confirmed source
G4  Cross-workspace superclass/template method works
G5  Template hook uniquely dispatches back to concrete override
G6  Ambiguous dispatch is PARTIAL, never guessed
G7  Dependency affects Navigation/Chain only, never Review/Write scope
G8  Real dual-project Windows regression + Release Gate are green
```

## Fixed development order

```text
Task 1 Contract
→ Task 2 Maven Source Verification
→ Task 3 Inheritance Navigation
→ Task 4 ChangeAnalysis / Chain Integration
→ Task 5 Real Business Regression
→ Task 6 Review Isolation
→ Task 7 Release Gate
```

Task 3 must not read sibling source until Task 2 has established VERIFIED Maven identity.