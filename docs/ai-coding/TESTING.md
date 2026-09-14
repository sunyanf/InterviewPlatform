# AI Coding 测试规范

## 1. 测试层级

```text
Unit
Integration
E2E
Eval
Load
```

---

# 2. AI 功能测试

必须覆盖：

```text
正常输出
空输出
非法 JSON
字段缺失
类型错误
超长输出
模型超时
模型失败
RAG 无结果
```

---

# 3. Prompt 测试

Prompt 变化需要：

```text
核心样本回归
边界样本
对抗样本
```

---

# 4. Eval

至少保存：

```text
input
expected
actual
score
model
prompt_version
timestamp
```

---

# 5. 禁止测试造假

绝对禁止：

```text
降低断言
删除 case
hardcode expected
```

为了让 CI 变绿。
