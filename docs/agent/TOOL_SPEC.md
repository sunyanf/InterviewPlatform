# Agent Tool 规范

## 1. Tool 原则

Tool 不是普通函数。

Tool 具有：

```text
权限
副作用
输入约束
输出 Contract
审计要求
```

---

# 2. Tool 分类

```text
READ
SEARCH
CALCULATE
WRITE
EXTERNAL
DESTRUCTIVE
```

---

# 3. 权限

Agent 默认只拥有：

```text
READ
SEARCH
CALCULATE
```

写入类 Tool：

```text
WRITE
```

必须有明确业务授权。

破坏性操作：

```text
DESTRUCTIVE
```

默认禁止 Agent 自主调用。

---

# 4. Tool Contract

必须定义：

```text
name
description
input_schema
output_schema
permission
side_effect
timeout
retry
audit
```

---

# 5. Tool 返回

Tool 返回必须：

```text
结构化
可验证
可观测
```

不要返回任意大段日志。
