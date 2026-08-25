package main

import (
    "errors"
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "codea-harness-tools/internal/nav"
    "codea-harness-tools/internal/schema"
    "codea-harness-tools/internal/workspace"
)

func validateWorkspaceHarnessConfig(data []byte) error {
    _, err := workspace.ValidateConfigYAML(".", data)
    return err
}

func runWorkspace(args []string) error {
    if len(args) == 0 { return errors.New("workspace requires verify") }
    switch args[0] {
    case "verify":
        fs := flag.NewFlagSet("workspace verify", flag.ContinueOnError)
        id := fs.String("id", "", "configured workspace dependency id")
        if err := fs.Parse(args[1:]); err != nil { return err }
        if fs.NArg() != 0 || strings.TrimSpace(*id) == "" { return errors.New("workspace verify requires --id") }
        result, err := loadWorkspaceVerification(strings.TrimSpace(*id))
        if err != nil {
            _ = writeJSONAndStatus(result, true)
            return err
        }
        return writeJSONAndStatus(result, true)
    default:
        return fmt.Errorf("unknown workspace action %q", args[0])
    }
}

func runWorkspaceNav(args []string) error {
    if len(args) == 0 { return errors.New("workspace nav action is required") }
    action := args[0]
    fs := flag.NewFlagSet("nav "+action, flag.ContinueOnError)
    workspaceID := fs.String("workspace", "", "configured workspace dependency id")
    from := fs.String("from", "", "qualified source method symbol")
    method := fs.String("method", "", "called inherited/superclass method")
    hook := fs.String("hook", "", "template hook method")
    concrete := fs.String("concrete", "", "optional concrete subclass")
    if err := fs.Parse(args[1:]); err != nil { return err }
    if fs.NArg() != 0 || strings.TrimSpace(*workspaceID) == "" || strings.TrimSpace(*from) == "" {
        return errors.New("workspace nav requires --workspace and --from")
    }

    verified, err := loadWorkspaceVerification(strings.TrimSpace(*workspaceID))
    if err != nil { return err }
    astPath, err := filepath.Abs(filepath.Join(".code-harness", "bin", "ast-grep.exe"))
    if err != nil { return err }
    currentRoot, err := filepath.Abs(".")
    if err != nil { return err }
    resolver := nav.WorkspaceInheritanceResolver{
        CurrentRoot: currentRoot,
        Dependency: verified,
        AstGrepPath: astPath,
    }

    var result nav.WorkspaceNavigationResult
    switch action {
    case "workspace-inherited":
        if strings.TrimSpace(*method) == "" { return errors.New("nav workspace-inherited requires --method") }
        result = resolver.ResolveInheritedCall(strings.TrimSpace(*from), strings.TrimSpace(*method))
    case "workspace-superclass-call":
        if strings.TrimSpace(*method) == "" { return errors.New("nav workspace-superclass-call requires --method") }
        result = resolver.ResolveSuperclassCall(strings.TrimSpace(*from), strings.TrimSpace(*method))
    case "workspace-template-dispatch":
        if strings.TrimSpace(*hook) == "" { return errors.New("nav workspace-template-dispatch requires --hook") }
        result = resolver.ResolveTemplateDispatch(strings.TrimSpace(*from), strings.TrimSpace(*hook), strings.TrimSpace(*concrete))
    default:
        return fmt.Errorf("unknown workspace nav action %q", action)
    }
    if err := writeJSONAndStatus(result, true); err != nil { return err }
    if result.Status != nav.NavigationComplete {
        if result.Limitation != nil && result.Limitation.Code != "" {
            return fmt.Errorf("%s: %s <- %s", result.Limitation.Code, result.Limitation.Symbol, result.Limitation.From)
        }
        return errors.New("workspace navigation PARTIAL")
    }
    return nil
}

func loadWorkspaceVerification(id string) (workspace.VerificationResult, error) {
    empty := workspace.VerificationResult{DependencyID: id}
    configPath := filepath.Join(".code-harness", "harness.yaml")
    schemaPath := filepath.Join(".code-harness", "contracts", "harness-config.schema.json")
    configData, err := os.ReadFile(configPath)
    if err != nil { return empty, fmt.Errorf("WORKSPACE_DEPENDENCY_NOT_CONFIGURED: read harness.yaml: %w", err) }
    schemaData, err := os.ReadFile(schemaPath)
    if err != nil { return empty, err }
    if err := schema.ValidateYAML(schemaData, configData); err != nil { return empty, err }
    deps, err := workspace.ValidateConfigYAML(".", configData)
    if err != nil { return empty, err }
    var selected *workspace.Dependency
    for i := range deps {
        if deps[i].ID == id { selected = &deps[i]; break }
    }
    if selected == nil {
        return empty, fmt.Errorf("WORKSPACE_DEPENDENCY_NOT_CONFIGURED: %s", id)
    }
    results := workspace.VerifyDirectMavenDependencies(".", []workspace.Dependency{*selected})
    if len(results) != 1 {
        return empty, fmt.Errorf("WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH: %s", id)
    }
    result := results[0]
    if result.Status != workspace.StatusVerified {
        code := strings.TrimSpace(result.Code)
        if code == "" { code = "WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH" }
        return result, fmt.Errorf("%s: %s", code, id)
    }
    return result, nil
}
