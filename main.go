package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/code-metrics/cli/internal/analyzer"
	"github.com/code-metrics/cli/internal/incremental"
	"github.com/code-metrics/cli/pkg/models"
)

func main() {
	var (
		repoPath      string
		format        string
		outputFile    string
		ciMode        bool
		trendAnalysis bool
		timeRange     string
		months        int
		jobs          int
		configFile    string
		minTokenLen   int
		highFanOut    int
		diff          string
	)

	flag.StringVar(&repoPath, "path", ".", "仓库路径")
	flag.StringVar(&format, "format", "terminal", "输出格式: terminal, json, html, csv")
	flag.StringVar(&outputFile, "output", "", "输出文件路径")
	flag.BoolVar(&ciMode, "ci", false, "CI模式，不输出进度信息，通过退出码返回结果")
	flag.BoolVar(&trendAnalysis, "trend", false, "启用趋势分析")
	flag.StringVar(&timeRange, "time-range", "", "时间范围，如 '2024-01-01' 或 '3 months ago'")
	flag.IntVar(&months, "months", 6, "分析最近N个月的Git历史")
	flag.IntVar(&jobs, "jobs", runtime.NumCPU(), "并发数")
	flag.StringVar(&configFile, "config", "", "配置文件路径")
	flag.IntVar(&minTokenLen, "min-token-len", 50, "重复代码检测的最小Token长度")
	flag.IntVar(&highFanOut, "high-fan-out", 15, "高耦合判定的扇出阈值")
	flag.StringVar(&diff, "diff", "", "增量分析模式，格式: commit1..commit2")

	flag.Parse()

	opts := &models.AnalyzerOptions{
		RepoPath:      repoPath,
		Format:        format,
		OutputFile:    outputFile,
		CIMode:        ciMode,
		TrendAnalysis: trendAnalysis,
		TimeRange:     timeRange,
		Months:        months,
		Jobs:          jobs,
		ConfigFile:    configFile,
		MinTokenLen:   minTokenLen,
		HighFanOut:    highFanOut,
	}

	if diff != "" {
		opts.Diff = diff
		runIncrementalAnalysis(opts)
	} else {
		analyzer.Run(opts)
	}
}

func runIncrementalAnalysis(opts *models.AnalyzerOptions) {
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
		fmt.Fprintf(os.Stderr, "错误: 不是Git仓库: %s\n", absPath)
		os.Exit(1)
	}

	parts := strings.Split(opts.Diff, "..")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		fmt.Fprintf(os.Stderr, "错误: --diff 参数格式错误，应为 commit1..commit2\n")
		os.Exit(1)
	}

	opts.RepoPath = absPath
	opts.DiffCommit1 = parts[0]
	opts.DiffCommit2 = parts[1]

	incremental.Run(opts)
}
