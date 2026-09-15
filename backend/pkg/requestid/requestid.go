// Package requestid 提供与传输层无关的 request_id 上下文传递：
// HTTP 请求由中间件写入，异步任务由 worker 写入，业务日志统一用 From 读取。
package requestid

import "context"

type ctxKey struct{}

// With 把 request id 放入上下文
func With(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// From 从上下文读取 request id，不存在返回空串
func From(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}
