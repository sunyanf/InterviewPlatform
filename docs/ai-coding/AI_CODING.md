# AI Coding 总规范

## 1. AI 的角色

Agent 是：

> 高级实现工程师 + 代码审查者 + 测试辅助者。

不是：

> 自动产品经理 + 自动架构师 + 自动发布管理员。

---

# 2. 默认工作模式

```text
Read
↓
Understand
↓
Plan
↓
Implement
↓
Test
↓
Review
↓
Verify
↓
Report
```

---

# 3. AI Coding 前提

Agent 必须先回答：

```text
我修改哪个模块？
模块负责什么？
不负责什么？
谁依赖它？
我需要改什么？
我不需要改什么？
```

---

# 4. 编码优先级

```text
正确性
>
安全性
>
可维护性
>
可测试性
>
性能
>
代码简洁
>
炫技
```

---

# 5. AI 不得自己扩大 Scope

任务只要求：

```text
增加 A
```

就不能默认修改：

```text
A + B + C + D
```

---

# 6. 先搜索再创建

在新建：

```text
service
repository
helper
util
component
```

之前必须搜索项目是否已有相同能力。

避免重复实现。

---

# 7. 优先修改现有代码

不要看到旧代码就重写。

先判断：

```text
能复用？
能局部修改？
是否有真实问题？
```

---

# 8. AI 生成代码也必须接受 Code Review

不能因为：

> “代码是 AI 写的。”

就降低 Review 标准。
