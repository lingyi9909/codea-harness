# P2：确认消息全文被 Git 清理的修正

复审基线 `eeffcad86aff04958d2a41bee9dc672430bd854e` 的 `git commit -F` 会应用Git消息清理：即使没有hook/并发，默认whitespace会去行尾空白，strip还会删除有效注释字符开头的正文。提交后发现差异太晚，不能满足已确认全文契约。

## 当前规则

- 展示并确认最终消息原文，使用text代码块；含行尾空格等不可见字符时在块外标行号/数量，说明本身不进入消息。
- 本次使用 `git commit --cleanup=verbatim -F <UTF-8无BOM消息文件>`。该标准Git选项只指定本次消息清理模式；不改用户/公司/仓库配置，不使用git -c覆盖，不跳hooks或签名。
- 预览前核对Git支持且项目明确规则允许原文模式；禁止时停止说明冲突，等待处理，不默默降级或改写全文。用户要调整文字则展示新全文、重新确认。
- commit-msg等hook仍可拒绝/改写；提交后照常查真实消息/树/父提交。被hook改写必须报告需复核，不重试/撤回。

## 独立真实 Git 对照（本地macOS，Git 2.39.5）

测试只创建新的C01临时仓库，配置只写该仓库local，不动开发仓库或全局配置。完整输入：

```text
fix(fixture): 展示并确认说明

# 保留的业务说明
; 自定义注释字符也是正文
正文尾部有空格
```

最后一行在真实测试输入中额外含两个行尾空格，此处按可读文本展示；精确输入由测试里的显式字符串给定。测试将该原文写入确认checkpoint，断言完整commit-body bytes与手工给定消息相等，并检查Git树、父提交、index/worktree、local配置与hooks未被改动。

| 生效配置（仅临时仓库） | 旧 `-F` | 新 `--cleanup=verbatim -F` |
|---|---|---|
| 未设置cleanup/注释字符（默认） | 行尾空白被清理，确认全文检查FAIL | 原始消息bytes一致，PASS |
| commit.cleanup=strip | #行及行尾空白被清理，FAIL | #与;行及行尾空白均保留，PASS |
| strip + core.commentChar=; | ;行及行尾空白被清理，FAIL | 全文原文保留，PASS |
| whitespace + core.commentChar=; | 行尾空白被清理，FAIL | 全文原文保留，PASS |

先用旧提交方式运行同一四组测试观察到4个子场景FAIL，再仅改变本次提交参数观察全部PASS。另有独立无flag strip负对照，必须触发 `commit message differs from confirmed full text`；不是把旧问题从验证器里忽略。默认的终止换行呈现差异契约保持，其余正文和空白均不得静默清理。

运行：`python3 tests/fixtures/codea-commit/test_fixtures.py`。相关方法为 `test_confirmed_text_survives_git_cleanup_and_comment_settings` 与 `test_implicit_cleanup_negative_control_changes_confirmed_message`；同套件保留hook拒绝/改写、签名失败和部分暂存对照。

这些是Git行为与验证器对照，不证明真实模型调用了该参数。公司Windows/Git版本、项目消息规则兼容、模型工具调用及TUI展示仍PENDING，见P14/P15内网验收。
