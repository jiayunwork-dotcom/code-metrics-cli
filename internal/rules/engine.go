package rules

import (
	"fmt"

	"github.com/code-metrics/cli/pkg/models"
)

type Engine struct {
	config    *models.RulesConfig
	rulesFile string
}

func NewEngine(rulesFile string, repoPath string) (*Engine, error) {
	targetFile := rulesFile
	if targetFile == "" {
		targetFile = repoPath + "/.code-metrics-rules.yml"
	}

	cfg, resolvedFile, err := LoadRulesFile(targetFile)
	if err != nil {
		return nil, err
	}

	if cfg == nil {
		return nil, nil
	}

	return &Engine{
		config:    cfg,
		rulesFile: resolvedFile,
	}, nil
}

func (e *Engine) IsEnabled() bool {
	return e != nil && e.config != nil
}

func (e *Engine) RulesFile() string {
	if e == nil {
		return ""
	}
	return e.rulesFile
}

func (e *Engine) Evaluate(report *models.Report) *models.CustomRulesResult {
	return e.evaluateForMode(report, nil, models.RuleModeFull)
}

func (e *Engine) EvaluateIncremental(report *models.IncrementalReport) *models.CustomRulesResult {
	return e.evaluateForMode(nil, report, models.RuleModeIncremental)
}

func (e *Engine) evaluateForMode(fullReport *models.Report, incReport *models.IncrementalReport, mode models.RuleMode) *models.CustomRulesResult {
	result := &models.CustomRulesResult{
		Enabled:     true,
		RulesFile:   e.rulesFile,
		TotalRules:  0,
		Groups:      []models.GroupResult{},
	}

	var metricValues map[string]interface{}
	var incCtx *IncrementalMetricContext

	if mode == models.RuleModeFull && fullReport != nil {
		metricValues = ExtractMetricValues(fullReport)
	} else if mode == models.RuleModeIncremental && incReport != nil {
		incCtx = ExtractIncrementalMetricValues(incReport)
		metricValues = incCtx.Values
	} else {
		metricValues = make(map[string]interface{})
	}

	for _, group := range e.config.Groups {
		groupResult := models.GroupResult{
			GroupName: group.Name,
			Logic:     group.Logic,
			Results:   []models.RuleResult{},
		}

		groupPassed := true
		if group.Logic == models.LogicAND {
			groupPassed = true
		} else {
			groupPassed = false
		}

		for _, rule := range group.Rules {
			result.TotalRules++
			ruleResult := evaluateRuleForMode(group.Name, rule, metricValues, incCtx, mode)
			groupResult.Results = append(groupResult.Results, ruleResult)

			switch ruleResult.Status {
			case models.RuleStatusPassed:
				result.PassedCount++
				if group.Logic == models.LogicOR && !groupPassed {
					groupPassed = true
				}
			case models.RuleStatusFailed:
				result.FailedCount++
				if ruleResult.Severity == models.SeverityError {
					result.HasErrors = true
				}
				if group.Logic == models.LogicAND && groupPassed {
					groupPassed = false
				}
			case models.RuleStatusSkipped:
				result.SkippedCount++
			}
		}

		groupResult.Passed = groupPassed
		result.Groups = append(result.Groups, groupResult)
	}

	return result
}

func ruleAppliesToMode(rule models.Rule, mode models.RuleMode) bool {
	if rule.Mode == "" {
		return true
	}
	return rule.Mode == mode
}

func evaluateRuleForMode(groupName string, rule models.Rule, metricValues map[string]interface{}, incCtx *IncrementalMetricContext, mode models.RuleMode) models.RuleResult {
	result := models.RuleResult{
		RuleName:  rule.Name,
		GroupName: groupName,
		Severity:  rule.Severity,
		Message:   rule.Message,
	}

	if !ruleAppliesToMode(rule, mode) {
		result.Status = models.RuleStatusSkipped
		result.SkipReason = fmt.Sprintf("规则仅适用于 %s 模式", rule.Mode)
		return result
	}

	parsed, err := ParseCondition(rule.Condition)
	if err != nil {
		result.Status = models.RuleStatusSkipped
		result.SkipReason = fmt.Sprintf("条件表达式解析错误: %v", err)
		return result
	}

	if err := ValidateConditionForMode(parsed, mode); err != nil {
		result.Status = models.RuleStatusSkipped
		result.SkipReason = err.Error()
		return result
	}

	passed, actual, err := EvaluateConditionWithContext(parsed, metricValues, incCtx)
	if err != nil {
		result.Status = models.RuleStatusSkipped
		result.SkipReason = err.Error()
		return result
	}

	displayField := parsed.Field
	if parsed.IsChange {
		displayField = fmt.Sprintf("change(%s)", parsed.ChangeField)
	}
	result.Actual = FormatActualValue(displayField, actual)

	if passed {
		result.Status = models.RuleStatusPassed
	} else {
		result.Status = models.RuleStatusFailed
	}

	return result
}

func ShouldExitWithCode3(customResult *models.CustomRulesResult) bool {
	if customResult == nil || !customResult.Enabled {
		return false
	}
	return customResult.HasErrors
}
