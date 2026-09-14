# ADR-0002：LLM Provider Adapter

## 状态

Accepted

## 决策

业务层只依赖内部 LLM Interface。

Provider 通过 Adapter 实现。

## 原因

- 避免业务代码绑定单一厂商；
- 支持模型切换；
- 便于测试；
- 便于成本优化。

## 影响

增加了一层抽象，但提高长期可维护性。
