# 数据模型设计

## 1. 核心表

```text
users
user_profiles

job_categories
jobs

resumes

interview_sessions
interview_questions
interview_answers

evaluation_results
interview_reports

knowledge_documents
knowledge_chunks

user_capabilities
learning_plans

llm_usages
```

---

# 2. users

```sql
id
email
password_hash
nickname
avatar_url
status
created_at
updated_at
```

---

# 3. jobs

```text
id
category_id
title
description
requirements
skills
source
source_url
status
created_at
updated_at
```

---

# 4. resumes

```text
id
user_id
file_url
file_name
parsed_content
structured_data
created_at
```

---

# 5. interview_sessions

```text
id
user_id
job_id
resume_id
interview_type
mode
status
config
started_at
ended_at
created_at
```

状态：

```text
INIT
READY
RUNNING
PAUSED
FINISHING
EVALUATING
COMPLETED
FAILED
```

---

# 6. interview_questions

```text
id
session_id
seq
question_type
question
difficulty
source
expected_points
metadata
created_at
```

---

# 7. interview_answers

```text
id
session_id
question_id
input_type
text_content
media_url
duration_ms
transcript
created_at
```

---

# 8. evaluation_results

```text
id
session_id
answer_id
overall_score
dimensions
strengths
weaknesses
evidence
recommendations
model_name
prompt_version
created_at
```

---

# 9. interview_reports

```text
id
session_id
overall_score
dimension_scores
summary
strengths
weaknesses
knowledge_gaps
learning_plan
next_actions
created_at
```

---

# 10. user_capabilities

```text
id
user_id
capability_code
score
confidence
evidence
updated_at
```

---

# 11. llm_usages

必须记录：

```text
provider
model
input_tokens
output_tokens
latency_ms
estimated_cost
prompt_version
session_id
created_at
```

目的：

- 成本分析；
- 性能分析；
- Prompt 迭代；
- 模型对比。
