#!/usr/bin/env python3
from pathlib import Path

FILES = [
    Path('.code-harness/AGENTS.md'),
    Path('.code-harness/bootstrap.md'),
    Path('.code-harness/agents/orchestrator.md'),
    Path('.code-harness/skills/review-code/SKILL.md'),
]

ACTIVE = '''# Codea Harness 1.8 普通 Review 主路径

普通 `/harness-review` 只执行 1.8 report-first 协议：固定入口先运行 `review start` 创建并回读 INCOMPLETE 报告，然后主 Agent 只通过 `codea-review` 完成 `prepare → (必要时等待真实下一用户选择后 select) → finish`。

- `runId` 必须来自本次入口，禁止“取最新 run”。
- 用户 target 是普通文本/结构化 `intent.target`，不得拼入 shell。
- 多链必须展示当前 `optionsHash` 与完整链菜单并结束当前 assistant turn；只有下一条真实用户选择可授权 `select`。HostTurn 只来自 OpenCode tool context，模型参数不得填写 session/message/userConfirmed。
- 只有 prepare 自动得到完整单链，或 select 成功后的 `scope.reads`，才是允许读取/提交的范围。跨 scope、源码 hash/range 变化必须 fail closed。
- 主 Agent 自己完成语义 Review；普通 1.8 Review 不要求旧 Reviewer 子 Agent、Certified ChangeAnalysis、ReviewUnit、RuleDispatch 或旧 finding certification。
- findings 为空也必须调用 `codea-review action=finish`。只有 finish 返回 `execution=COMPLETE` 才能称“评审完成”。`coverage=PARTIAL` 时结论必须是 `UNDETERMINED`，报告保留所有已知 gap。
- 任一步失败或用户未选择时保留已经存在的 INCOMPLETE 报告，不自动重启整轮，不进入修复代码。

对普通 1.8 Review，以上合同是本文件唯一可执行 Review 协议。

## 历史 Review 协议与其他既有能力

下面保留的版本化内容用于旧 run / 升级兼容和非 Review 能力。**其中 1.7 及更早的 ordinary Review begin / Reviewer / certify / ReviewUnit / RuleDispatch / report-review 步骤全部是历史说明，不得用于新的 1.8 `/harness-review`。** Test、Debug、Fix、API Doc、Chain、Upgrade 等非 ordinary Review 规则若未被 1.8 设计修改，继续有效。

'''


def split_frontmatter(text: str):
    if not text.startswith('---\n'):
        return '', text
    end = text.find('\n---\n', 4)
    if end < 0:
        raise RuntimeError('unterminated YAML frontmatter')
    end += len('\n---\n')
    return text[:end], text[end:]


for path in FILES:
    text = path.read_text(encoding='utf-8')
    if text.startswith(ACTIVE):
        text = text[len(ACTIVE):]
    frontmatter, body = split_frontmatter(text)
    normalized = frontmatter + ACTIVE + body
    if normalized != path.read_text(encoding='utf-8'):
        path.write_text(normalized, encoding='utf-8', newline='\n')
        print(f'PATCHED {path}')
