package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// MockProvider Mock LLM Provider，用于开发和测试
// 返回预定义的结构化输出，不调用真实 API
type MockProvider struct{}

// NewMockProvider 创建 Mock Provider
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// Name 返回 Provider 名称
func (p *MockProvider) Name() string {
	return "mock"
}

// Chat 返回 Mock 响应
func (p *MockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// 根据最后一条消息内容返回不同的 mock 结果
	lastMsg := ""
	if len(req.Messages) > 0 {
		lastMsg = req.Messages[len(req.Messages)-1].Content
	}

	// 简历解析 mock 输出
	if contains(lastMsg, "简历") || contains(lastMsg, "resume") {
		mockResume := map[string]interface{}{
			"name":  "张三",
			"email": "zhangsan@example.com",
			"phone": "13800138000",
			"education": []map[string]string{
				{"school": "某某大学", "major": "计算机科学与技术", "degree": "本科"},
			},
			"work_experience": []map[string]string{
				{"company": "某某科技", "position": "后端工程师", "duration": "2021-至今"},
			},
			"skills":          []string{"Go", "MySQL", "Redis", "Kafka", "微服务", "Docker"},
			"summary":         "5年后端开发经验，熟悉高并发系统设计",
			"target_position": "后端工程师",
		}
		data, _ := json.Marshal(mockResume)
		return &ChatResponse{
			Content: string(data),
			Model:   "mock",
		}, nil
	}

	// 回答分析 mock 输出
	if contains(lastMsg, "请分析候选人对下面问题的回答") {
		mockAnalysis := map[string]interface{}{
			"claims":         []string{"goroutine 是用户态轻量级协程", "channel 遵循 CSP 模型"},
			"correct_points": []string{"说出了 goroutine 轻量级和用户态调度的特点"},
			"wrong_points":   []string{"channel 一定是线程安全的说法不准确"},
			"missing_points": []string{"未提到 channel 的关闭和 select 机制"},
			"knowledge_gaps": []string{"channel 底层实现"},
		}
		data, _ := json.Marshal(mockAnalysis)
		return &ChatResponse{
			Content:      string(data),
			Model:        "mock",
			InputTokens:  80,
			OutputTokens: 150,
		}, nil
	}

	// 学习计划 mock 输出
	if contains(lastMsg, "生成学习计划") {
		mockPlan := map[string]interface{}{
			"strengths":  []string{"goroutine 与 channel 概念表述准确", "能结合项目说明高并发手段"},
			"weaknesses": []string{"channel 关闭与 select 机制掌握不足", "回答缺少结构化展开"},
			"focus_areas": []map[string]interface{}{
				{
					"topic":       "Go 并发原语深入",
					"reason":      "知识缺口：channel 关闭语义与 select 机制（对应 correctness 维度不足）",
					"suggestions": []string{"阅读 Go 官方文档 channel 章节", "实现生产者-消费者练习题", "总结 select 超时与非阻塞用法"},
				},
				{
					"topic":       "回答结构化表达",
					"reason":      "communication 维度可提升：先结论后论据",
					"suggestions": []string{"练习 STAR 法则作答", "每题先给结论再分点展开"},
				},
			},
			"next_training": map[string]interface{}{
				"focus":                   "Go 并发与 GMP 调度",
				"suggested_question_type": "technical",
				"suggested_difficulty":    "medium",
				"suggested_topics":        []string{"channel 关闭语义", "GMP 调度模型", "sync 包原语"},
			},
		}
		data, _ := json.Marshal(mockPlan)
		return &ChatResponse{
			Content:      string(data),
			Model:        "mock",
			InputTokens:  250,
			OutputTokens: 220,
		}, nil
	}

	// 整场评估 mock 输出（注意：在出题检测之前，prompt 含"评估官"不含"面试官"）
	if contains(lastMsg, "整场问答记录进行评估") {
		mockEvaluation := map[string]interface{}{
			"dimensions": map[string]interface{}{
				"correctness":   70,
				"depth":         60,
				"logic":         75,
				"communication": 80,
			},
			"evidence": []map[string]interface{}{
				{
					"question_seq": 1,
					"issue":        "未提到 channel 的关闭和 select 机制",
					"evidence":     "候选人只说明了 channel 用于通信，未涉及关闭语义",
					"reference":    "go-official-docs",
				},
			},
			"recommendations": []string{
				"深入学习 channel 的底层实现与关闭语义",
				"回答时先给结论再展开论据",
				"结合项目经验说明高并发手段",
			},
		}
		data, _ := json.Marshal(mockEvaluation)
		return &ChatResponse{
			Content:      string(data),
			Model:        "mock",
			InputTokens:  200,
			OutputTokens: 180,
		}, nil
	}

	// 追问决策 mock 输出
	if contains(lastMsg, "判断是否需要追问") {
		mockFollowUp := map[string]interface{}{
			"should_follow_up": true,
			"reason":           "候选人对 channel 的回答不够深入，存在明显知识缺口",
			"question":         "你刚才提到 channel，能具体说说带缓冲和不带缓冲 channel 的区别吗？",
			"target_gap":       "channel 底层机制",
		}
		data, _ := json.Marshal(mockFollowUp)
		return &ChatResponse{
			Content:      string(data),
			Model:        "mock",
			InputTokens:  80,
			OutputTokens: 80,
		}, nil
	}

	// 面试开场白 mock 输出（注意：必须在面试官/面试题检测之前）
	if contains(lastMsg, "面试开场白") {
		return &ChatResponse{
			Content:      "你好，欢迎参加本次岗位面试。我是今天的面试官，我们会围绕岗位相关技能进行交流。让我们先从第一道题开始：",
			Model:        "mock",
			InputTokens:  40,
			OutputTokens: 60,
		}, nil
	}

	// 面试出题 mock 输出
	if contains(lastMsg, "面试题") || contains(lastMsg, "面试官") {
		mockQuestions := map[string]interface{}{
			"questions": []map[string]interface{}{
				{
					"question":   "请介绍一下 Go 语言中 goroutine 和线程的区别，以及 channel 的使用场景。",
					"type":       "technical",
					"difficulty": "medium",
					"expected_points": []string{
						"goroutine 是用户态轻量级协程，线程是内核态",
						"goroutine 初始栈小，可动态增长",
						"channel 用于 goroutine 间通信",
					},
				},
				{
					"question":   "请描述一次你参与高并发系统设计的经历，你是如何保证系统稳定性的？",
					"type":       "project",
					"difficulty": "medium",
					"expected_points": []string{
						"有具体场景和量化指标",
						"提到限流、熔断、降级等手段",
						"有复盘和改进",
					},
				},
				{
					"question":   "你如何理解微服务架构的优缺点？在什么场景下会选择微服务？",
					"type":       "technical",
					"difficulty": "easy",
					"expected_points": []string{
						"优点：独立部署、团队自治",
						"缺点：分布式复杂性、运维成本",
						"小团队小业务不需要微服务",
					},
				},
			},
		}
		data, _ := json.Marshal(mockQuestions)
		return &ChatResponse{
			Content:      string(data),
			Model:        "mock",
			InputTokens:  50,
			OutputTokens: 200,
		}, nil
	}

	// 默认返回简单响应
	return &ChatResponse{
		Content:      fmt.Sprintf(`{"response": "mock response for: %s"}`, truncate(lastMsg, 50)),
		Model:        "mock",
		InputTokens:  10,
		OutputTokens: 20,
	}, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
