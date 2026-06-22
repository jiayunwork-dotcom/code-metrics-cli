package rules

import (
	"fmt"
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
	Field    string
	Operator Operator
	Value    string
	IsString bool
}

var supportedFields = map[string]bool{
	"complexity.max":         true,
	"complexity.avg":         true,
	"duplication.rate":       true,
	"duplication.max_block":  true,
	"dependency.cycles":      true,
	"dependency.max_fan_out": true,
	"scoring.total":          true,
	"scoring.grade":          true,
}

var stringFields = map[string]bool{
	"scoring.grade": true,
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

	field := strings.TrimSpace(expr[:opIdx])
	value := strings.TrimSpace(expr[opIdx+len(foundOp):])

	if !supportedFields[field] {
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
		Field:    field,
		Operator: foundOp,
		Value:    value,
		IsString: isString,
	}, nil
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

func FormatActualValue(field string, actual interface{}) string {
	if actual == nil {
		return "N/A"
	}
	if stringFields[field] {
		return fmt.Sprintf("%v", actual)
	}
	if f, ok := actual.(float64); ok {
		return fmt.Sprintf("%.2f", f)
	}
	return fmt.Sprintf("%v", actual)
}

func EvaluateCondition(parsed *ParsedCondition, metricValues map[string]interface{}) (bool, interface{}, error) {
	actual, exists := metricValues[parsed.Field]
	if !exists {
		return false, nil, fmt.Errorf("指标字段 %s 未找到（可能对应分析模块未执行）", parsed.Field)
	}

	if parsed.IsString {
		return evaluateStringCondition(parsed, actual)
	}
	return evaluateNumericCondition(parsed, actual)
}

func evaluateNumericCondition(parsed *ParsedCondition, actual interface{}) (bool, interface{}, error) {
	var actualNum float64
	switch v := actual.(type) {
	case float64:
		actualNum = v
	case int:
		actualNum = float64(v)
	default:
		return false, actual, fmt.Errorf("字段 %s 的值不是数值类型: %T", parsed.Field, actual)
	}

	threshold, err := strconv.ParseFloat(parsed.Value, 64)
	if err != nil {
		return false, actual, fmt.Errorf("解析阈值失败: %s", parsed.Value)
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
