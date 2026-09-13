from pathlib import Path

exec(Path('.code-harness/tools-runtime/task170_t5_patch2.py').read_text(), {})
for name in [
    '.code-harness/skills/analyze-change/SKILL.md',
    '.code-harness/agents/reviewer.md',
    '.code-harness/agents/orchestrator.md',
    '.code-harness/AGENTS.md',
    '.code-harness/tools/README.md',
]:
    p = Path(name)
    p.write_text(p.read_text().rstrip() + '\n')
