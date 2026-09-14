# AI Eval 规范

## 1. Dataset

目录：

```text
evals/
├── datasets/
│   ├── golden/
│   ├── regression/
│   ├── adversarial/
│   └── edge_cases/
```

---

# 2. Golden

人工高质量答案。

用于：

> 判断模型是否退化。

---

# 3. Regression

曾经出现过问题的样本。

用于：

> 防止 Bug 重新出现。

---

# 4. Adversarial

故意攻击：

- Prompt Injection；
- 跑题；
- 虚假信息；
- 超长输入；
- 恶意输入。

---

# 5. Edge Cases

测试：

- 空答案；
- 无 RAG；
- LLM timeout；
- 非法 JSON；
- 极端评分。

---

# 6. Eval 发布门槛

重大 Prompt / Model 变化至少比较：

```text
正确性
相关性
结构化输出成功率
评分一致性
幻觉
成本
延迟
```
