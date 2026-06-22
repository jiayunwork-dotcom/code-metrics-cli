package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/code-metrics/cli/pkg/models"
	"gopkg.in/yaml.v3"
)

func DefaultConfig() *models.Config {
	return &models.Config{
		QualityGates: models.QualityGates{
			MaxAvgComplexity:   10.0,
			MaxDuplicationRate: 15.0,
			AllowCycles:        false,
		},
	}
}

func LoadConfig(path string) (*models.Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return cfg, nil
		}
		path = filepath.Join(cwd, ".code-metrics.yml")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func CheckQualityGates(report *models.Report, gates models.QualityGates) (bool, []string) {
	var violations []string
	passed := true

	if report.Complexity != nil && report.Complexity.Average > gates.MaxAvgComplexity {
		violations = append(violations,
			fmt.Sprintf("平均圈复杂度超标: %.2f > %.2f",
				report.Complexity.Average, gates.MaxAvgComplexity))
		passed = false
	}

	if report.Duplication != nil && report.Duplication.DuplicationRate > gates.MaxDuplicationRate {
		violations = append(violations,
			fmt.Sprintf("重复率超标: %.1f%% > %.1f%%",
				report.Duplication.DuplicationRate, gates.MaxDuplicationRate))
		passed = false
	}

	if !gates.AllowCycles && report.Dependency != nil && report.Dependency.CycleCount > 0 {
		violations = append(violations,
			fmt.Sprintf("存在循环依赖: 检测到 %d 个循环", report.Dependency.CycleCount))
		passed = false
	}

	return passed, violations
}
