package metrics

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// counterSeries 单组标签值的计数器序列（单调递增，允许小数用于 cost）
type counterSeries struct {
	labelValues []string
	value       atomic.Uint64 // math.Float64bits 编码，CAS 更新
}

func (s *counterSeries) add(v float64) {
	if v <= 0 {
		return
	}
	for {
		old := s.value.Load()
		newV := math.Float64frombits(old) + v
		if s.value.CompareAndSwap(old, math.Float64bits(newV)) {
			return
		}
	}
}

func (s *counterSeries) get() float64 {
	return math.Float64frombits(s.value.Load())
}

// CounterVec 带标签计数器
type CounterVec struct {
	name       string
	help       string
	labelNames []string

	mu     sync.Mutex
	values map[string]*counterSeries
}

// Inc 计数 +1，标签值数量必须与定义一致
func (c *CounterVec) Inc(labelValues ...string) { c.Add(1, labelValues...) }

// Add 增加任意非负值
func (c *CounterVec) Add(v float64, labelValues ...string) {
	c.mu.Lock()
	key := labelKey(labelValues)
	s, ok := c.values[key]
	if !ok {
		s = &counterSeries{labelValues: append([]string(nil), labelValues...)}
		c.values[key] = s
	}
	c.mu.Unlock()
	s.add(v)
}

func (c *CounterVec) write(b *strings.Builder) {
	fmtHelpType(b, c.name, c.help, "counter")
	c.mu.Lock()
	series := make([]*counterSeries, 0, len(c.values))
	for _, s := range c.values {
		series = append(series, s)
	}
	c.mu.Unlock()
	sort.Slice(series, func(i, j int) bool {
		return labelKey(series[i].labelValues) < labelKey(series[j].labelValues)
	})
	for _, s := range series {
		b.WriteString(c.name)
		b.WriteString(formatLabels(c.labelNames, s.labelValues))
		b.WriteByte(' ')
		b.WriteString(formatFloat(s.get()))
		b.WriteByte('\n')
	}
}

// histSeries 单组标签值的直方图样本。
// counts[i] 为落入 buckets[i] 及其之前各桶的累计样本数（Prometheus 语义），
// 末位 counts[len(buckets)] 为 +Inf 桶，即样本总数。
type histSeries struct {
	labelValues []string
	counts      []uint64
	sum         float64

	mu sync.Mutex
}

// HistogramVec 带标签直方图
type HistogramVec struct {
	name       string
	help       string
	labelNames []string
	buckets    []float64 // 升序上界（不含 +Inf）

	mu     sync.Mutex
	values map[string]*histSeries
}

// Observe 记录一个样本
func (h *HistogramVec) Observe(v float64, labelValues ...string) {
	h.mu.Lock()
	key := labelKey(labelValues)
	s, ok := h.values[key]
	if !ok {
		s = &histSeries{labelValues: append([]string(nil), labelValues...), counts: make([]uint64, len(h.buckets)+1)}
		h.values[key] = s
	}
	h.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, bound := range h.buckets {
		if v <= bound {
			s.counts[i]++ // 每个上界 >= v 的桶都累加（累计桶语义）
		}
	}
	s.counts[len(h.buckets)]++ // +Inf 桶永远累加
	s.sum += v
}

func (h *HistogramVec) write(b *strings.Builder) {
	fmtHelpType(b, h.name, h.help, "histogram")
	h.mu.Lock()
	series := make([]*histSeries, 0, len(h.values))
	for _, s := range h.values {
		series = append(series, s)
	}
	h.mu.Unlock()
	sort.Slice(series, func(i, j int) bool {
		return labelKey(series[i].labelValues) < labelKey(series[j].labelValues)
	})

	for _, s := range series {
		s.mu.Lock()
		labels := formatLabels(h.labelNames, s.labelValues)
		for i, bound := range h.buckets {
			writeBucket(b, h.name, labels, formatFloat(bound), s.counts[i])
		}
		writeBucket(b, h.name, labels, "+Inf", s.counts[len(h.buckets)])
		b.WriteString(h.name)
		b.WriteString("_sum")
		b.WriteString(labels)
		b.WriteByte(' ')
		b.WriteString(formatFloat(s.sum))
		b.WriteByte('\n')
		b.WriteString(h.name)
		b.WriteString("_count")
		b.WriteString(labels)
		b.WriteByte(' ')
		b.WriteString(strconv.FormatUint(s.counts[len(h.buckets)], 10))
		b.WriteByte('\n')
		s.mu.Unlock()
	}
}

// writeBucket 写一行 _bucket；labels 已含其他标签时把 le 合并进花括号内
func writeBucket(b *strings.Builder, name, labels, le string, count uint64) {
	b.WriteString(name)
	b.WriteString("_bucket")
	switch {
	case labels == "":
		b.WriteString(`{le="`)
		b.WriteString(le)
		b.WriteString(`"}`)
	default:
		b.WriteString(strings.TrimSuffix(labels, "}"))
		b.WriteString(`,le="`)
		b.WriteString(le)
		b.WriteString(`"}`)
	}
	b.WriteByte(' ')
	b.WriteString(strconv.FormatUint(count, 10))
	b.WriteByte('\n')
}
