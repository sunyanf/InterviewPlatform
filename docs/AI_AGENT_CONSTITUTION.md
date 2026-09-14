# AI Agent 宪法说明

本文件是 `AGENTS.md` 的说明版。

真正具有最高规则效力的是仓库根目录：

```text
AGENTS.md
```

---

## 核心思想

### 第一层

产品和架构是人的责任。

### 第二层

Agent 负责高质量实现。

### 第三层

运行时事实由代码和数据库保证。

### 第四层

LLM 处理复杂自然语言，但不直接拥有核心状态。

---

## 为什么这样设计

AI Coding 越来越强，项目真正的风险从：

> “AI 写不出代码”

逐渐转变为：

> “AI 写出了大量代码，但代码是否正确、是否符合架构、是否长期可维护？”

因此本项目更重视：

```text
Context
Constraint
Review
Testing
Evaluation
Versioning
Traceability
```

而不是单纯追求生成速度。
