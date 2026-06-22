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
	result := &models.CustomRulesResult{
		Enabled:    true,
		RulesFile:  e.rulesFile,
		TotalRules: 0,
		Groups:     []models.GroupResult{},
	}

	metricValues := ExtractMetricValues(report)

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
			ruleResult := evaluateRule(group.Name, rule, metricValues)
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

func evaluateRule(groupName string, rule models.Rule, metricValues map[string]interface{}) models.RuleResult {
	result := models.RuleResult{
		RuleName:  rule.Name,
		GroupName: groupName,
		Severity:  rule.Severity,
		Message:   rule.Message,
	}

	parsed, err := ParseCondition(rule.Condition)
	if err != nil {
		result.Status = models.RuleStatusSkipped
		result.SkipReason = fmt.Sprintf("条件表达式解析错误: %v", err)
		return result
	}

	passed, actual, err := EvaluateCondition(parsed, metricValues)
	if err != nil {
		result.Status = models.RuleStatusSkipped
		result.SkipReason = err.Error()
		return result
	}

	result.Actual = FormatActualValue(parsed.Field, actual)

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
