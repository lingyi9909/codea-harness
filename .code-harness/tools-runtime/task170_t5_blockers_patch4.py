from pathlib import Path

ROOT = Path('.code-harness/tools-runtime')
path = ROOT / 'cmd/codea-dcep-tools/review_context_entry_170.go'
p = path.read_text(encoding='utf-8')

old = '''\treturn reviewprogress.Read(".", strings.TrimSpace(*runID))
}'''
new = '''\tstate, err := reviewprogress.Read(".", strings.TrimSpace(*runID))
\tif err != nil {
\t\t// Legacy review dispatches predate Runtime-owned review progress. Only an
\t\t// absent progress file is allowed to route to legacy behavior. If a
\t\t// progress artifact exists but is malformed/stale, Read returns a
\t\t// different error and 1.7 fails closed instead of downgrading.
\t\tif errors.Is(err, os.ErrNotExist) {
\t\t\treturn reviewprogress.State{}, nil
\t\t}
\t\treturn reviewprogress.State{}, err
\t}
\treturn state, nil
}'''
if p.count(old) != 1:
    raise SystemExit(f'review dispatch state anchor count={p.count(old)}')
p = p.replace(old, new, 1)
path.write_text(p, encoding='utf-8', newline='\n')
print('TASK170_T5_LEGACY_DISPATCH_COMPAT_PATCH_APPLIED')
