package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/code-metrics/cli/internal/config"
	"github.com/code-metrics/cli/internal/complexity"
	"github.com/code-metrics/cli/internal/dependency"
	"github.com/code-metrics/cli/internal/duplication"
	"github.com/code-metrics/cli/internal/git"
	"github.com/code-metrics/cli/internal/metrics"
	"github.com/code-metrics/cli/internal/output"
	"github.com/code-metrics/cli/internal/scoring"
	"github.com/code-metrics/cli/internal/trend"
	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

func Run(opts *models.AnalyzerOptions) {
	absPath, err := filepath.Abs(opts.RepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 路径解析失败: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 路径不存在: %s\n", absPath)
		os.Exit(1)
	}

	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "警告: 不是Git仓库: %s\n", absPath)
		fmt.Fprintf(os.Stderr, "Git相关分析将被跳过\n")
	}

	configPath := opts.ConfigFile
	if configPath == "" {
		configPath = filepath.Join(absPath, ".code-metrics.yml")
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 配置文件加载失败: %v\n", err)
	}

	startTime := time.Now()
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "开始分析仓库: %s\n", utils.FormatFilepath(absPath))
		fmt.Fprintf(os.Stderr, "并发数: %d\n\n", opts.Jobs)
	}

	report := &models.Report{
		RepoPath:    absPath,
		GeneratedAt: time.Now(),
	}

	files, err := utils.WalkFiles(absPath, utils.DefaultSkipDirs())
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 文件遍历失败: %v\n", err)
		os.Exit(1)
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "发现 %d 个源文件\n", len(files))
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[1/8] 分析基本度量... ")
	}
	report.Metrics = metrics.Analyze(files, absPath, opts.Jobs)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[2/8] 分析圈复杂度... ")
	}
	report.Complexity = complexity.Analyze(files, absPath, opts.Jobs)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[3/8] 检测重复代码... ")
	}
	report.Duplication = duplication.Analyze(files, absPath, opts.MinTokenLen, opts.Jobs)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[4/8] 分析依赖关系... ")
	}
	report.Dependency = dependency.Analyze(files, absPath, opts.HighFanOut, opts.Jobs)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[5/8] 分析Git变更热点... ")
	}
	report.GitHotspots = git.AnalyzeHotspots(absPath, report.Complexity, opts)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[6/8] 分析作者贡献... ")
	}
	report.Contributors = git.AnalyzeContributors(absPath, opts)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[7/8] 计算技术债评分... ")
	}
	report.TechDebt = scoring.Calculate(report)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if opts.TrendAnalysis {
		if !opts.CIMode {
			fmt.Fprintf(os.Stderr, "[8/8] 趋势分析 (这可能需要几分钟)... ")
		}
		report.Trend = trend.Analyze(absPath, opts)
		if !opts.CIMode {
			fmt.Fprintf(os.Stderr, "完成\n")
		}
	} else if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[8/8] 趋势分析已跳过 (使用 --trend 启用)\n")
	}

	duration := time.Since(startTime)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "\n分析完成, 耗时: %s\n\n", duration.Round(time.Millisecond))
	}

	exitCode := 0
	if opts.CIMode {
		passed, violations := config.CheckQualityGates(report, cfg.QualityGates)
		report.QualityGates = &models.QualityGateResult{
			Passed:     passed,
			Violations: violations,
		}
		if !passed {
			exitCode = 1
		}
	}

	output.Write(report, opts, cfg)
	os.Exit(exitCode)
}
