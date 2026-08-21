---
name: discover-api
description: 在受控导航边界内发现 Spring Controller endpoint 与 changed-mode API targets。
version: 1
---

# Discover API

## 输入

```text
target = <Controller> | <Controller.method> | changed
scope = repository-relative source scope
runId
```

## 输出

结构化 target 列表，每项至少包含：

```text
controller
method(optional)
path
httpMethod
sourcePath
lineStart
lineEnd
selectionOrigin = EXPLICIT | AUTO_SINGLE | USER_SELECTED
```

## Controller / Method 发现

1. 用 `get-symbol-info` 定位显式 Controller 或 Controller.method。
2. 用 `find-by-annotation` 搜索 Spring mapping annotation：`RequestMapping/GetMapping/PostMapping/PutMapping/DeleteMapping/PatchMapping`。
3. 合并 class-level + method-level mapping 得到最终 path/httpMethod。
4. 无法唯一确定 symbol/path/httpMethod 时 fail closed；不得猜。

## changed 模式

必须复用 Review Change Set / ChangeAnalysis，不另造 diff 语义：

```text
merge-base(baseRef, HEAD) → HEAD committed
+ staged
+ unstaged
+ untracked
```

读取 `affectedControllers`：

```text
0 -> NO_API_TARGET -> STOP
1 -> AUTO_SINGLE
2+ -> WAITING_API_SELECTION
```

2+ 时优先 native multi-select；fallback 仅允许编号列表、`1,3`、`ALL`。不得默认 `ALL`。空选择/取消必须 STOP。

## 安全

- 只读。
- 不调用 Maven/Test/DB。
- 不修改项目文件。
- 不直接执行 raw ast-grep rule/pattern/regex/query。
- Selection 不构成任何写操作审批。
