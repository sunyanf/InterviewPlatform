# AI Coding 工作流

## Phase 1：理解

读取：

- AGENTS.md
- 项目记忆
- 任务
- 架构
- 相关模块

输出：

```text
目标
范围
非目标
影响
```

---

## Phase 2：探索

搜索：

```text
相关代码
测试
数据库
API
Prompt
Contract
```

---

## Phase 3：设计

形成：

```text
方案 A
方案 B
推荐方案
风险
测试策略
```

简单任务可以直接执行。

复杂任务先给计划。

---

## Phase 4：实现

原则：

```text
小步修改
小步测试
小步提交
```

---

## Phase 5：验证

至少：

```bash
go test ./...
go test -race ./...
go vet ./...
```

并根据任务补：

```text
integration
e2e
eval
load test
```

---

## Phase 6：Review

检查：

```text
业务
架构
并发
事务
安全
AI 输出
Prompt
错误处理
日志
```

---

## Phase 7：汇报

必须提供：

```text
完成项
修改文件
测试
风险
未完成项
```
