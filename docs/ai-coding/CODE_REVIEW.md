# AI 代码 Review 清单

## 1. 正确性

- 业务规则正确？
- 边界条件？
- 空值？
- 错误处理？

## 2. Go

- context 正确？
- goroutine 生命周期？
- race？
- defer？
- error wrapping？
- connection close？

## 3. 数据库

- SQL 正确？
- 索引？
- 事务？
- 隔离级别？
- N+1？
- 超大查询？

## 4. Redis

- TTL？
- 热 key？
- 缓存一致性？
- 原子性？

## 5. AI

- Structured Output？
- Schema Validation？
- Prompt Version？
- Token？
- Timeout？
- Retry？
- Provider 解耦？

## 6. Agent

- 是否违反状态机？
- 是否无限循环？
- Tool 权限？
- 是否过度依赖 LLM？

## 7. RAG

- source？
- metadata？
- relevance？
- no-result？
- hallucination？

## 8. Security

- Prompt Injection？
- 越权？
- 文件上传？
- 敏感日志？

## 9. Observability

- request_id？
- trace？
- error metrics？
- latency？

## 10. 测试

- happy path？
- edge case？
- failure path？
- regression？
