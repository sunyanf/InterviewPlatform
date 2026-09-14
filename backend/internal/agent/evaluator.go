package agent

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// 评估维度（与 docs/AI_DATA_CONTRACTS.md #5 一致）
const (
	DimCorrectness   = "correctness"   // 技术正确性
	DimDepth         = "depth"         // 技术深度
	DimLogic         = "logic"         // 逻辑思维
	DimCommunication = "communication" // 表达
)

// validDimensions 维度白名单（归一化时忽略未知维度）
var validDimensions = map[string]bool{
	DimCorrectness:   true,
	DimDepth:         true,
	DimLogic:         true,
	DimCommunication: true,
}

// EvaluatedQA 评估输入的单题数据
type EvaluatedQA struct {
	Seq            int
	Question       string
	QuestionType   string
	ExpectedPoints []string
	AnswerText     string
	Analysis       *AnswerAnalysis
}

// EvaluateInput 整场评估输入
type EvaluateInput struct {
	JobTitle      string
	InterviewType string
	QA            []EvaluatedQA
	// Knowledge 知识库参考片段（可选，RAG 召回）
	Knowledge []string
}

// EvidenceItem 评估证据（每个扣分/加分项尽量带证据，见 docs/EVALUATION.md #3）
type EvidenceItem struct {
	QuestionSeq int    `json:"question_seq"`
	Issue       string `json:"issue"`
	Evidence    string `json:"evidence"`
	Reference   string `json:"reference"`
}

// EvaluationOutput 评估输出（AI 数据 Contract）
// 注意：不含总分——总分由业务代码按 Rubric 权重确定性计算
type EvaluationOutput struct {
	Dimensions      map[string]float64 `json:"dimensions"`
	Evidence        []EvidenceItem     `json:"evidence"`
	Recommendations []string           `json:"recommendations"`
	PromptVersion   string             `json:"prompt_version"`
	Model           string             `json:"model"`
}

// Evaluate 整场评估：LLM 产出维度分/证据/建议，业务侧校验归一化
func (a *Agent) Evaluate(ctx context.Context, in EvaluateInput) (*EvaluationOutput, error) {
	if len(in.QA) == 0 {
		return nil, fmt.Errorf("evaluate requires at least one answered question")
	}

	// 逐题拼接问答与分析
	var qa strings.Builder
	for _, item := range in.QA {
		qa.WriteString(fmt.Sprintf(`
[题 %d] 类型：%s
问题：%s
参考要点：%s
候选人回答：%s
回答分析：正确点 %s；错误点 %s；缺失点 %s；知识缺口 %s
`, item.Seq, item.QuestionType, item.Question,
			joinOrNone(item.ExpectedPoints),
			joinOrNone([]string{item.AnswerText}),
			joinOrNone(analysisField(item.Analysis, "correct")),
			joinOrNone(analysisField(item.Analysis, "wrong")),
			joinOrNone(analysisField(item.Analysis, "missing")),
			joinOrNone(analysisField(item.Analysis, "gaps"))))
	}

	// 知识库参考资料（可选）
	knowledgeSection := ""
	if len(in.Knowledge) > 0 {
		var refs strings.Builder
		for i, k := range in.Knowledge {
			refs.WriteString(fmt.Sprintf("[%d] %s\n", i+1, k))
		}
		knowledgeSection = fmt.Sprintf(`
评估参考资料（评分依据必须来自候选人回答与以下资料，不得编造）：
%s
`, refs.String())
	}

	prompt := fmt.Sprintf(`你是一个严格的技术面试评估官。请基于以下整场问答记录进行评估。

岗位：%s
面试类型：%s
%s
问答记录：%s
要求：
1. 只返回 JSON，不要包含任何解释文字
2. JSON 结构：{"dimensions":{"correctness":0,"depth":0,"logic":0,"communication":0},"evidence":[{"question_seq":0,"issue":"","evidence":"","reference":""}],"recommendations":[""]}
3. dimensions 四个维度都是 0-100 的整数：correctness(技术正确性)、depth(技术深度)、logic(逻辑思维)、communication(表达)
4. evidence 每项必须引用候选人回答原文片段作为证据，reference 可为空或知识来源
5. recommendations 给出 2-5 条具体改进建议
6. 评分必须基于问答记录中的事实，不得臆测`,
		in.JobTitle, in.InterviewType, knowledgeSection, qa.String())

	content, err := a.chat(ctx, buildSystemPrompt("你是一个客观严格、基于证据的面试评估官", PromptEvaluator), prompt, 0.3)
	if err != nil {
		return nil, err
	}

	var out EvaluationOutput
	if err := unmarshal(content, &out); err != nil {
		return nil, err
	}
	normalizeEvaluation(&out)
	if err := validateEvaluation(&out); err != nil {
		return nil, err
	}

	out.PromptVersion = PromptEvaluator
	out.Model = a.llm.Name()
	return &out, nil
}

// normalizeEvaluation 评估输出业务归一化（AGENTS.md #11）
func normalizeEvaluation(out *EvaluationOutput) {
	// 维度分 clamp 0-100、非法值置 NaN 待校验剔除
	for k, v := range out.Dimensions {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			out.Dimensions[k] = math.NaN()
			continue
		}
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		out.Dimensions[k] = math.Round(v)
	}
	// 只保留白名单维度
	for k := range out.Dimensions {
		if !validDimensions[k] {
			delete(out.Dimensions, k)
		}
	}
	// evidence 非法项剔除
	validEvidence := out.Evidence[:0]
	for _, e := range out.Evidence {
		if strings.TrimSpace(e.Evidence) == "" {
			continue
		}
		validEvidence = append(validEvidence, e)
	}
	out.Evidence = validEvidence
	// recommendations 清洗空项
	validRecs := out.Recommendations[:0]
	for _, r := range out.Recommendations {
		if strings.TrimSpace(r) != "" {
			validRecs = append(validRecs, r)
		}
	}
	out.Recommendations = validRecs
}

// validateEvaluation 评估输出业务校验：四个维度必须全部有效（缺维度视为评估无效，可重跑）
func validateEvaluation(out *EvaluationOutput) error {
	for _, dim := range []string{DimCorrectness, DimDepth, DimLogic, DimCommunication} {
		v, ok := out.Dimensions[dim]
		if !ok || math.IsNaN(v) {
			return fmt.Errorf("evaluation output missing or invalid dimension %q", dim)
		}
	}
	return nil
}

// analysisField 提取分析字段（nil 安全）
func analysisField(analysis *AnswerAnalysis, field string) []string {
	if analysis == nil {
		return nil
	}
	switch field {
	case "correct":
		return analysis.CorrectPoints
	case "wrong":
		return analysis.WrongPoints
	case "missing":
		return analysis.MissingPoints
	case "gaps":
		return analysis.KnowledgeGaps
	}
	return nil
}

// joinOrNone 切片拼接，空时返回"无"
func joinOrNone(items []string) string {
	joined := strings.Join(items, "；")
	if strings.TrimSpace(joined) == "" {
		return "无"
	}
	return joined
}
