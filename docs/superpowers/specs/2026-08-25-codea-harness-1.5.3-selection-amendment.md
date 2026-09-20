# Codea Harness 1.5.3 — Review Selection Amendment

This amendment is normative for the approved `2026-08-25-codea-harness-1.5.3-chain-reliability-editing-design.md`.

## Single-option auto execution

User selection is required only when there are **two or more** valid selectable Controllers/Chains for the current decision point.

```text
selectable count = 0
→ no selection UI; follow the intent's normal fallback (for plain review: FULL remains available when completeness is proven)

selectable count = 1
→ no selection UI
→ Runtime emits deterministic AUTO_SINGLE selection
→ execute that Controller/Chain scope directly

selectable count >= 2
→ show structured multi-select or numbered fallback
→ Runtime verifies selected IDs against the exact optionsHash
```

This rule applies both to:

- plain `harness review` after completeness/certification when Chain-targeted execution has exactly one valid business Chain;
- explicit Service/downstream targeting when exactly one upstream Controller/Chain is valid.

It does **not** allow silently skipping the Changed Controller / EntryPoint Completeness Gate. A single visible Chain may only auto-execute after the Runtime has proven that the relevant inventory is complete for the current intent.

The previous design subsection saying an exactly-one-Chain case should still show a lightweight choice is superseded by this amendment.
