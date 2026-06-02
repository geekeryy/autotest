package evalagent

import (
	"context"
	"encoding/json"
	"time"

	"autotest/internal/logx"
)

// Evaluator runs periodic self-evaluations of the AI assistant.
type Evaluator struct {
	repo *Repository
}

// NewEvaluator constructs an Evaluator.
func NewEvaluator(repo *Repository) *Evaluator {
	return &Evaluator{repo: repo}
}

// RunEvaluation computes an evaluation report for the given time period.
func (e *Evaluator) RunEvaluation(ctx context.Context, since time.Time) (*EvalReport, error) {
	report := &EvalReport{
		PeriodStart: since,
		PeriodEnd:   time.Now(),
	}

	// 1. Routing accuracy: success outcomes / total routing logs
	totalRouting, correctRouting, err := e.repo.AggregateRoutingAccuracy(ctx, since)
	if err == nil && totalRouting > 0 {
		report.RoutingAccuracy = float64(correctRouting) / float64(totalRouting)
	}

	// 2. Tool efficiency: average hops
	totalOutcomes, successOutcomes, avgHops, err := e.repo.AggregateOutcomes(ctx, since)
	if err == nil {
		report.ToolEfficiency = avgHops
		report.ToolSuccessRate = 0
		if totalOutcomes > 0 {
			report.ToolSuccessRate = float64(successOutcomes) / float64(totalOutcomes)
		}
	}

	// 3. User satisfaction: positive feedback / total feedback
	totalFeedback, positiveFeedback, err := e.repo.AggregateFeedback(ctx, since)
	if err == nil && totalFeedback > 0 {
		report.UserSatisfaction = float64(positiveFeedback) / float64(totalFeedback)
	}

	// 4. Skill coverage: placeholder (would need session-level data)
	report.SkillCoverage = 0

	// 5. Generate suggestions based on metrics.
	suggestions := e.generateSuggestions(report)
	report.Suggestions = mustJSON(suggestions)

	// 6. Persist the report.
	if err := e.repo.Create(ctx, report); err != nil {
		logx.Warn("eval: persist report", "err", err)
		return report, err
	}

	return report, nil
}

// generateSuggestions produces actionable suggestions based on evaluation metrics.
func (e *Evaluator) generateSuggestions(report *EvalReport) []string {
	var suggestions []string

	if report.RoutingAccuracy < 0.7 {
		suggestions = append(suggestions, "路由准确率偏低（<70%），建议检查 Planner 提示词或增加训练数据")
	}
	if report.ToolEfficiency > 5 {
		suggestions = append(suggestions, "平均工具调用轮数偏高（>5），建议优化工具描述或合并高频工具序列为技能")
	}
	if report.UserSatisfaction < 0.6 {
		suggestions = append(suggestions, "用户满意度偏低（<60%），建议分析低分反馈中的共性问题")
	}
	if report.ToolSuccessRate < 0.8 {
		suggestions = append(suggestions, "工具成功率偏低（<80%），建议检查失败工具的参数 Schema 或前置条件")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "各项指标正常，继续保持")
	}
	return suggestions
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	if b == nil {
		return json.RawMessage("[]")
	}
	return b
}
