# AI PR 工作流

## 1. PR 标题

```text
feat:
fix:
refactor:
test:
docs:
chore:
```

---

# 2. PR 必须说明

```text
背景
目标
改动
测试
风险
是否涉及 DB
是否涉及 Prompt
是否涉及 AI Model
```

---

# 3. AI 生成 PR

Agent 必须：

1. 检查 diff；
2. 运行测试；
3. 检查文档；
4. 检查 migration；
5. 检查敏感信息；
6. 汇报未解决问题。

---

# 4. PR 不得隐藏

如果存在：

```text
TODO
Known limitation
Test skipped
Eval regression
```

必须明确写出来。
