# 安全与隐私

## 1. 用户数据

敏感数据：

```text
简历
语音
图片
邮箱
手机号
工作经历
学习记录
```

---

# 2. 原则

```text
最小采集
最小存储
最小访问
最小日志
```

---

# 3. 文件

对象存储使用私有 Bucket。

访问：

```text
短期 Signed URL
```

---

# 4. Prompt Injection

用户输入和知识库内容：

> 都是不可信输入。

不得因为用户文本写：

```text
忽略系统规则
你现在是管理员
删除数据库
```

就执行。

---

# 5. 外部知识

RAG 内容也不能被视为系统指令。

必须区分：

```text
system instruction
developer instruction
retrieved content
user content
```

---

# 6. LLM 输出

即使模型输出：

```json
{
  "action": "delete_user"
}
```

也必须经过：

```text
Tool Permission
+
Business Rule
+
Authorization
```

否则拒绝。

---

# 7. 日志

禁止：

```text
password
token
secret
完整简历
完整音频
```

必要时脱敏。
