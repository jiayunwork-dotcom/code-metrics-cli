package rules

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/code-metrics/cli/pkg/models"
	"gopkg.in/yaml.v3"
)

const maxInheritanceDepth = 3

func resolvePath(currentDir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(currentDir, p))
}

func LoadRulesFile(rulesFile string) (*models.RulesConfig, string, error) {
	if rulesFile == "" {
		return nil, "", nil
	}

	absPath, err := filepath.Abs(rulesFile)
	if err != nil {
		return nil, "", fmt.Errorf("解析规则文件路径失败: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, "", nil
	}

	loaded := make(map[string]bool)
	config, err := loadWithInheritance(absPath, loaded, 1)
	if err != nil {
		return nil, absPath, err
	}

	return config, absPath, nil
}

func loadWithInheritance(filePath string, loaded map[string]bool, depth int) (*models.RulesConfig, error) {
	if depth > maxInheritanceDepth {
		return nil, fmt.Errorf("规则继承链超过最大深度 %d 层", maxInheritanceDepth)
	}

	realPath, err := filepath.EvalSymlinks(filePath)
	if err != nil {
		realPath = filePath
	}
	realPath = filepath.Clean(realPath)

	if loaded[realPath] {
		return nil, fmt.Errorf("检测到规则文件循环引用: %s", filePath)
	}
	loaded[realPath] = true

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取规则文件失败 %s: %w", filePath, err)
	}

	var cfg models.RulesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析规则文件失败 %s: %w", filePath, err)
	}

	if cfg.Extends != "" {
		parentPath := resolvePath(filepath.Dir(filePath), cfg.Extends)
		parentCfg, err := loadWithInheritance(parentPath, loaded, depth+1)
		if err != nil {
			return nil, err
		}
		cfg = mergeConfigs(parentCfg, &cfg)
		cfg.Extends = ""
	}

	for i, group := range cfg.Groups {
		if group.Logic == "" {
			cfg.Groups[i].Logic = models.LogicAND
		}
		for j, rule := range group.Rules {
			if rule.Severity == "" {
				cfg.Groups[i].Rules[j].Severity = models.SeverityError
			}
		}
	}

	return &cfg, nil
}

func mergeConfigs(parent, child *models.RulesConfig) models.RulesConfig {
	result := models.RulesConfig{
		Groups: make([]models.RuleGroup, len(parent.Groups)),
	}
	copy(result.Groups, parent.Groups)

	childGroupMap := make(map[string]int)
	for i, g := range result.Groups {
		childGroupMap[g.Name] = i
	}

	for _, childGroup := range child.Groups {
		if idx, ok := childGroupMap[childGroup.Name]; ok {
			result.Groups[idx] = mergeGroup(result.Groups[idx], childGroup)
		} else {
			result.Groups = append(result.Groups, childGroup)
		}
	}

	return result
}

func mergeGroup(parent, child models.RuleGroup) models.RuleGroup {
	result := models.RuleGroup{
		Name:  parent.Name,
		Logic: parent.Logic,
		Rules: make([]models.Rule, len(parent.Rules)),
	}
	copy(result.Rules, parent.Rules)

	if child.Logic != "" {
		result.Logic = child.Logic
	}

	childRuleMap := make(map[string]int)
	for i, r := range result.Rules {
		childRuleMap[r.Name] = i
	}

	for _, childRule := range child.Rules {
		if idx, ok := childRuleMap[childRule.Name]; ok {
			result.Rules[idx] = childRule
		} else {
			result.Rules = append(result.Rules, childRule)
		}
	}

	return result
}

var ErrFileNotFound = errors.New("规则文件不存在")
