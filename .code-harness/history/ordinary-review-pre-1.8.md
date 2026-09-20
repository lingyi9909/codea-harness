# Ordinary Review protocol history before Codea Harness 1.8

This file is historical reference only. It is not an active Agent, command, bootstrap, or Skill instruction and must not be loaded to execute a new ordinary `/harness-review`.

Before 1.8, ordinary Review evolved through flows that included `review begin`, snapshot/certification phases, `ReviewUnit`, `RuleDispatch`, finding certification, `report review`, and a terminal `REPORT SUCCEEDED` marker. Some versions also used `codea-reviewer-submit` to transport semantic proposals.

Those ordinary-Review orchestration rules are superseded by the 1.8 report-first path:

`review start → codea-review prepare → optional real next-user selection → codea-review finish`

The old material is retained only to explain legacy run artifacts and upgrade compatibility. It has no authority over a new 1.8 Review. Non-Review capabilities such as Test, Debug, Fix, API documentation, Chain management, initialization, and Upgrade keep their own active instructions in the normal Codea Harness files.
