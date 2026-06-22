package rules

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/code-metrics/cli/pkg/models"
)

type Operator string

const (
	OpGreaterThan        Operator = ">"
	OpLessThan           Operator = "<"
	OpEqual              Operator = "=="
	OpNotEqual           Operator = "!="
	OpGreaterThanOrEqual Operator = ">="
	OpLessThanOrEqual    Operator = "<="
)

type ParsedCondition struct {
	Field      string
	Operator   Operator
	Value      string
	IsString   bool
	IsChange   bool
	ChangeField string
}

var fullModeFields = map[string]bool{
	"complexity.max":         true,
	"complexity.avg":         true,
	"duplication.rate":       true,
	"duplication.max_block":  true,
	"dependency.cycles":      true,
	"dependency.max_fan_out": true,
	"scoring.total":          true,
	"scoring.grade":          true,
}

var incrementalModeFields = map[string]bool{
	"incremental.complexity_diff":     true,
	"incremental.new_high_risk":       true,
	"incremental.new_duplication_rate": true,
	"incremental.new_cycles":          true,
	"incremental.changed_files":       true,
}

var stringFields = map[string]bool{
	"scoring.grade": true,
}

var changePattern = regexp.MustCompile(`^change\(([^)]+)\)$`)

func isFullModeField(field string) bool {
	return fullModeFields[field]
}

func isIncrementalModeField(field string) bool {
	return incrementalModeFields[field]
}

func isSupportedField(field string, mode models.RuleMode) bool {
	if mode == models.RuleModeIncremental {
		return isIncrementalModeField(field)
	}
	if mode == models.RuleModeFull {
		return isFullModeField(field)
	}
	return isFullModeField(field) || isIncrementalModeField(field)
}

func ParseCondition(expr string) (*ParsedCondition, error) {
	expr = strings.TrimSpace(expr)

	opOrder := []Operator{OpGreaterThanOrEqual, OpLessThanOrEqual, OpEqual, OpNotEqual, OpGreaterThan, OpLessThan}

	var foundOp Operator
	var opIdx int = -1

	for _, op := range opOrder {
		idx := strings.Index(expr, string(op))
		if idx != -1 {
			if opIdx == -1 || idx < opIdx {
				foundOp = op
				opIdx = idx
			}
		}
	}

	if opIdx == -1 {
		return nil, fmt.Errorf("无法解析条件表达式中的操作符: %s", expr)
	}

	fieldRaw := strings.TrimSpace(expr[:opIdx])
	value := strings.TrimSpace(expr[opIdx+len(foundOp):])

	isChange := false
	changeField := ""
	field := fieldRaw

	if m := changePattern.FindStringSubmatch(fieldRaw); m != nil {
		isChange = true
		changeField = strings.TrimSpace(m[1])
		field = changeField
	}

	if !isFullModeField(field) && !isIncrementalModeField(field) {
		return nil, fmt.Errorf("不支持的指标字段: %s", field)
	}

	isString := stringFields[field]

	if !isString {
		value = strings.Trim(value, "\"'")
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return nil, fmt.Errorf("数值字段 %s 的值必须是数字，实际: %s", field, value)
		}
	} else {
		if foundOp != OpEqual && foundOp != OpNotEqual {
			return nil, fmt.Errorf("字符串字段 %s 仅支持 == 和 != 操作符", field)
		}
		value = strings.Trim(value, "\"'")
	}

	return &ParsedCondition{
		Field:       field,
		Operator:    foundOp,
		Value:       value,
		IsString:    isString,
		IsChange:    isChange,
		ChangeField: changeField,
	}, nil
}

func ValidateConditionForMode(parsed *ParsedCondition, mode models.RuleMode) error {
	if mode == models.RuleModeFull && parsed.IsChange {
		return fmt.Errorf("change() 函数仅在增量模式下可用")
	}

	if mode == models.RuleModeIncremental && isFullModeField(parsed.Field) && !parsed.IsChange {
		return fmt.Errorf("全量指标 %s 在增量模式下不可用", parsed.Field)
	}

	if mode == models.RuleModeFull && isIncrementalModeField(parsed.Field) {
		return fmt.Errorf("增量指标 %s 在全量模式下不可用", parsed.Field)
	}

	return nil
}

func ExtractMetricValues(report *models.Report) map[string]interface{} {
	values := make(map[string]interface{})

	if report.Complexity != nil {
		maxComplexity := 0
		for _, f := range report.Complexity.TopComplex {
			if f.Complexity > maxComplexity {
				maxComplexity = f.Complexity
			}
		}
		values["complexity.max"] = float64(maxComplexity)
		values["complexity.avg"] = report.Complexity.Average
	}

	if report.Duplication != nil {
		values["duplication.rate"] = report.Duplication.DuplicationRate
		maxBlock := 0
		for _, b := range report.Duplication.TopDuplicates {
			if b.TokenLength > maxBlock {
				maxBlock = b.TokenLength
			}
		}
		values["duplication.max_block"] = float64(maxBlock)
	}

	if report.Dependency != nil {
		values["dependency.cycles"] = float64(report.Dependency.CycleCount)
		maxFanOut := 0
		for _, f := range report.Dependency.FanInOut {
			if f.FanOut > maxFanOut {
				maxFanOut = f.FanOut
			}
		}
		values["dependency.max_fan_out"] = float64(maxFanOut)
	}

	if report.TechDebt != nil {
		values["scoring.total"] = report.TechDebt.Score
		values["scoring.grade"] = report.TechDebt.Grade
	}

	return values
}

type IncrementalMetricContext struct {
	Values    map[string]interface{}
	OldValues map[string]float64
	NewValues map[string]float64
}

func ExtractIncrementalMetricValues(report *models.IncrementalReport) *IncrementalMetricContext {
	ctx := &IncrementalMetricContext{
		Values:    make(map[string]interface{}),
		OldValues: make(map[string]float64),
		NewValues: make(map[string]float64),
	}

	ctx.Values["incremental.changed_files"] = float64(len(report.ChangedFiles))

	if report.Complexity != nil {
		ctx.Values["incremental.complexity_diff"] = float64(report.Complexity.TotalDiff)
		ctx.Values["incremental.new_high_risk"] = float64(report.Complexity.NewHighRiskCount)

		oldTotal := 0
		newTotal := 0
		countWithOld := 0
		countWithNew := 0
		for _, fd := range report.Complexity.FileDiffs {
			oldTotal += fd.OldComplexity
			newTotal += fd.NewComplexity
			if fd.OldComplexity > 0 {
				countWithOld++
			}
			countWithNew++
		}

		if countWithOld > 0 {
			oldAvg := float64(oldTotal) / float64(countWithOld)
			ctx.OldValues["complexity.avg"] = oldAvg
			ctx.OldValues["complexity.total"] = float64(oldTotal)
		} else {
			ctx.OldValues["complexity.avg"] = 0
			ctx.OldValues["complexity.total"] = 0
		}
		if countWithNew > 0 {
			newAvg := float64(newTotal) / float64(countWithNew)
			ctx.NewValues["complexity.avg"] = newAvg
			ctx.NewValues["complexity.total"] = float64(newTotal)
		} else {
			ctx.NewValues["complexity.avg"] = 0
			ctx.NewValues["complexity.total"] = 0
		}
	}

	if report.Duplication != nil {
		ctx.Values["incremental.new_duplication_rate"] = report.Duplication.NewDuplicationRate
	}

	if report.Dependency != nil {
		ctx.Values["incremental.new_cycles"] = float64(report.Dependency.NewCycleCount)
	}

	return ctx
}

func FormatActualValue(field string, actual interface{}) string {
	if actual == nil {
		return "N/A"
	}
	if stringFields[field] {
		return fmt.Sprintf("%v", actual)
	}
	if f, ok := actual.(float64); ok {
		if math.IsInf(f, 1) {
			return "+∞"
		}
		return fmt.Sprintf("%.2f", f)
	}
	return fmt.Sprintf("%v", actual)
}

func EvaluateCondition(parsed *ParsedCondition, metricValues map[string]interface{}) (bool, interface{}, error) {
	return EvaluateConditionWithContext(parsed, metricValues, nil)
}

func EvaluateConditionWithContext(parsed *ParsedCondition, metricValues map[string]interface{}, incCtx *IncrementalMetricContext) (bool, interface{}, error) {
	var actual interface{}
	var err error

	if parsed.IsChange {
		actual, err = computeChange(parsed.ChangeField, incCtx)
		if err != nil {
			return false, nil, err
		}
	} else {
		var exists bool
		actual, exists = metricValues[parsed.Field]
		if !exists {
			return false, nil, fmt.Errorf("指标字段 %s 未找到（可能对应分析模块未执行）", parsed.Field)
		}
	}

	if parsed.IsString {
		return evaluateStringCondition(parsed, actual)
	}
	return evaluateNumericCondition(parsed, actual)
}

func computeChange(field string, incCtx *IncrementalMetricContext) (float64, error) {
	if incCtx == nil {
		return 0, fmt.Errorf("增量上下文为空，无法计算 change(%s)", field)
	}

	oldVal, oldOk := incCtx.OldValues[field]
	newVal, newOk := incCtx.NewValues[field]
	if !oldOk || !newOk {
		return 0, fmt.Errorf("字段 %s 没有足够的新旧数据来计算变化率", field)
	}

	if oldVal == 0 && newVal == 0 {
		return 0, nil
	}

	if oldVal == 0 {
		return math.Inf(1), nil
	}

	return (newVal - oldVal) / oldVal * 100, nil
}

func evaluateNumericCondition(parsed *ParsedCondition, actual interface{}) (bool, interface{}, error) {
	var actualNum float64
	switch v := actual.(type) {
	case float64:
		actualNum = v
	case int:
		actualNum = float64(v)
	case int64:
		actualNum = float64(v)
	default:
		return false, actual, fmt.Errorf("字段 %s 的值不是数值类型: %T", parsed.Field, actual)
	}

	threshold, err := strconv.ParseFloat(parsed.Value, 64)
	if err != nil {
		return false, actual, fmt.Errorf("解析阈值失败: %s", parsed.Value)
	}

	if math.IsInf(actualNum, 1) {
		return false, actualNum, nil
	}

	var result bool
	switch parsed.Operator {
	case OpGreaterThan:
		result = actualNum > threshold
	case OpLessThan:
		result = actualNum < threshold
	case OpEqual:
		result = actualNum == threshold
	case OpNotEqual:
		result = actualNum != threshold
	case OpGreaterThanOrEqual:
		result = actualNum >= threshold
	case OpLessThanOrEqual:
		result = actualNum <= threshold
	default:
		return false, actual, fmt.Errorf("未知操作符: %s", parsed.Operator)
	}

	return result, actualNum, nil
}

func evaluateStringCondition(parsed *ParsedCondition, actual interface{}) (bool, interface{}, error) {
	actualStr := fmt.Sprintf("%v", actual)
	threshold := parsed.Value

	var result bool
	switch parsed.Operator {
	case OpEqual:
		result = actualStr == threshold
	case OpNotEqual:
		result = actualStr != threshold
	default:
		return false, actual, fmt.Errorf("字符串不支持操作符: %s", parsed.Operator)
	}

	return result, actualStr, nil
}

var gradeRank = map[string]int{
	"A": 5,
	"B": 4,
	"C": 3,
	"D": 2,
	"E": 1,
	"F": 0,
}

var conditionPattern = regexp.MustCompile(`^\s*([\w.]+)\s*(>=|<=|==|!=|>|<)\s*(.+?)\s*$`)
