from pathlib import Path

ROOT = Path('.code-harness/tools-runtime')
path = ROOT / 'cmd/codea-dcep-tools/review_context_command_170.go'
p = path.read_text(encoding='utf-8')

old = '''func verifyReviewContextArtifactUse170(runID string, a analysisruntime.ChangeAnalysis, units reviewunit.Manifest, dispatch reviewrules.Manifest) error {
\tstate, err := reviewprogress.Read(".", runID); if err != nil { return err }
\tif state.ProtocolVersion != reviewprogress.Protocol170 { return nil }'''
new = '''func verifyReviewContextArtifactUse170(runID string, a analysisruntime.ChangeAnalysis, units reviewunit.Manifest, dispatch reviewrules.Manifest) error {
\tstate, err := reviewprogress.Read(".", runID)
\tif err != nil {
\t\t// Pre-1.7 Reviewer certification has no Runtime-owned progress artifact.
\t\t// Only that absence is legacy. Existing-but-invalid progress must remain
\t\t// fail-closed so a 1.7 run can never downgrade through corruption.
\t\tif errors.Is(err, os.ErrNotExist) {
\t\t\treturn nil
\t\t}
\t\treturn err
\t}
\tif state.ProtocolVersion != reviewprogress.Protocol170 { return nil }'''
if p.count(old) != 1:
    raise SystemExit(f'finding certification legacy anchor count={p.count(old)}')
p = p.replace(old, new, 1)
path.write_text(p, encoding='utf-8', newline='\n')
print('TASK170_T5_LEGACY_FINDING_CERT_PATCH_APPLIED')
