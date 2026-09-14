# Prompt Injection 防护

## 1. 不可信内容

以下都是不可信内容：

```text
用户回答
用户简历
JD
RAG 文档
上传文件
网页内容
```

---

# 2. 权限隔离

不可信内容不能改变：

```text
System Instruction
Developer Instruction
Tool Permission
业务规则
```

---

# 3. Tool 安全

即使模型生成：

```json
{
  "tool": "delete_user"
}
```

也必须经过：

```text
Tool Permission
+
Authorization
+
Business Validation
```

---

# 4. 输出过滤

工具和模型返回的文本不能直接：

```text
执行 SQL
执行 Shell
修改生产
```

除非有显式、受控、安全的执行路径。
