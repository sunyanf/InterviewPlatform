# Agent 上下文协议

## 1. 第一层：宪法

```text
AGENTS.md
```

## 2. 第二层：项目长期记忆

```text
docs/PROJECT_MEMORY.md
```

## 3. 第三层：相关架构

```text
docs/ARCHITECTURE.md
docs/architecture/*
```

## 4. 第四层：相关模块

目标目录附近的：

```text
AGENTS.md
README.md
```

## 5. 第五层：任务

```text
docs/tasks/TASK-xxxx.md
```

## 6. 第六层：代码

只搜索与任务相关的：

```text
implementation
tests
migrations
prompt
contract
```

---

# 2. 上下文最小化原则

不要一次读取整个仓库。

目标：

> 加载完成任务所需要的最少上下文。

---

# 3. 搜索原则

优先搜索：

```text
接口名
结构体名
表名
API Path
事件名
Prompt 名
测试名
```

而不是漫无目的读取文件。

---

# 4. 不确定时

优先：

```text
搜索代码
搜索文档
搜索测试
搜索 ADR
```

再做推断。

---

# 5. 上下文冲突

如果看到：

```text
文档 A：规则 X
代码 B：规则 Y
```

不得自行选择。

必须：

```text
指出冲突
分析影响
必要时请求决策
```
