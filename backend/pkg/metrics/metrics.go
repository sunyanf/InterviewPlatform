// Package metrics 提供零第三方依赖的进程内指标（Counter/Histogram）与
// Prometheus 文本格式暴露（/metrics）。MVP 阶段不引入 client_golang；
// 多实例部署时由 Prometheus 分别抓取各实例 /metrics 即可。
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Registry 指标注册表（协程安全）
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*CounterVec
	hists    map[string]*HistogramVec
}

// New 创建空注册表
func New() *Registry {
	return &Registry{
		counters: make(map[string]*CounterVec),
		hists:    make(map[string]*HistogramVec),
	}
}

// Counter 获取或创建带标签计数器；同名指标的 help/标签名必须一致
func (r *Registry) Counter(name, help string, labelNames ...string) *CounterVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		if !sameLabels(c.labelNames, labelNames) {
			panic(fmt.Sprintf("metrics: counter %q redefined with different labels", name))
		}
		return c
	}
	c := &CounterVec{name: name, help: help, labelNames: labelNames, values: make(map[string]*counterSeries)}
	r.counters[name] = c
	return c
}

// Histogram 获取或创建带标签直方图；buckets 必须为升序正数
func (r *Registry) Histogram(name, help string, buckets []float64, labelNames ...string) *HistogramVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.hists[name]; ok {
		if !sameLabels(h.labelNames, labelNames) {
			panic(fmt.Sprintf("metrics: histogram %q redefined with different labels", name))
		}
		return h
	}
	cp := append([]float64(nil), buckets...)
	sort.Float64s(cp)
	h := &HistogramVec{name: name, help: help, labelNames: labelNames, buckets: cp, values: make(map[string]*histSeries)}
	r.hists[name] = h
	return h
}

// Handler 返回 Prometheus 文本格式抓取端点
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = io.WriteString(w, r.Expose())
	})
}

// Expose 输出全部指标的文本表示
func (r *Registry) Expose() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	names := make([]string, 0, len(r.counters))
	for name := range r.counters {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r.counters[name].write(&b)
	}
	names = names[:0]
	for name := range r.hists {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r.hists[name].write(&b)
	}
	return b.String()
}

func sameLabels(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// labelKey 序列化为稳定的 map key（值数量固定，简单连接即可）
func labelKey(values []string) string {
	return strings.Join(values, "\x00")
}

func escapeLabelValue(v string) string {
	v = strings.ReplaceAll(v, "\\", `\\`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}

func formatLabels(names, values []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, 0, len(names))
	for i, n := range names {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		parts = append(parts, fmt.Sprintf("%s=%q", n, escapeLabelValue(v)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// fmtHelpType 输出 HELP/TYPE 两行
func fmtHelpType(b *strings.Builder, name, help, typ string) {
	b.WriteString("# HELP ")
	b.WriteString(name)
	b.WriteByte(' ')
	b.WriteString(help)
	b.WriteByte('\n')
	b.WriteString("# TYPE ")
	b.WriteString(name)
	b.WriteByte(' ')
	b.WriteString(typ)
	b.WriteByte('\n')
}
