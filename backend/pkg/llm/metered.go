package llm

import (
	"context"
	"errors"
	"time"

	"ai-interview-platform/pkg/metrics"
)

// ErrStreamingNotSupported 被装饰的 Provider 不支持流式调用
var ErrStreamingNotSupported = errors.New("llm provider does not support streaming")

// MeteredProvider 在真实 Provider 外层记录调用量、延迟、token 与估算费用。
// 不改变任何请求/响应语义；被装饰 Provider 同时实现 Streamer 时透传流式能力
// （流式 token 用量无法在分片层统计，仅记录建连成功/失败与建连延迟）。
type MeteredProvider struct {
	inner    Provider
	priceIn  float64 // 美元 / token（由配置的每 1K token 单价换算）
	priceOut float64
}

// NewMeteredProvider 包装 Provider；pricePer1K 为每 1000 token 美元单价，0 表示不计费
func NewMeteredProvider(inner Provider, priceInputPer1K, priceOutputPer1K float64) *MeteredProvider {
	return &MeteredProvider{
		inner:    inner,
		priceIn:  priceInputPer1K / 1000,
		priceOut: priceOutputPer1K / 1000,
	}
}

// Name 返回底层 Provider 名称
func (m *MeteredProvider) Name() string { return m.inner.Name() }

// Chat 执行调用并记录指标
func (m *MeteredProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()
	resp, err := m.inner.Chat(ctx, req)
	elapsed := time.Since(start).Seconds()

	providerName := m.inner.Name()
	reqModel := req.Model
	if reqModel == "" {
		reqModel = "unspecified"
	}
	status := "success"
	if err != nil {
		status = "error"
	}
	metrics.LLMRequests.Inc(providerName, reqModel, status)
	metrics.LLMDuration.Observe(elapsed, providerName, reqModel)

	// token 与费用按 Provider 实际返回的 model 名记账（请求计数仍记请求 model，避免重复）
	if err == nil && resp != nil {
		model := resp.Model
		if model == "" {
			model = reqModel
		}
		if resp.InputTokens > 0 {
			metrics.LLMTokens.Add(float64(resp.InputTokens), providerName, model, "input")
			metrics.LLMCost.Add(float64(resp.InputTokens)*m.priceIn, providerName, model)
		}
		if resp.OutputTokens > 0 {
			metrics.LLMTokens.Add(float64(resp.OutputTokens), providerName, model, "output")
			metrics.LLMCost.Add(float64(resp.OutputTokens)*m.priceOut, providerName, model)
		}
	}
	return resp, err
}

// StreamChat 透传流式调用，仅记录建连结果与延迟（流式用量统计留给账单侧）
func (m *MeteredProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	streamer, ok := m.inner.(Streamer)
	if !ok {
		return nil, ErrStreamingNotSupported
	}
	start := time.Now()
	ch, err := streamer.StreamChat(ctx, req)
	status := "success"
	if err != nil {
		status = "error"
	}
	model := req.Model
	if model == "" {
		model = "unspecified"
	}
	metrics.LLMRequests.Inc(m.inner.Name(), model, "stream_"+status)
	metrics.LLMDuration.Observe(time.Since(start).Seconds(), m.inner.Name(), model)
	return ch, err
}
