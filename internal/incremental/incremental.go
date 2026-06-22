package incremental

import (
	"fmt"
	"os"
	"time"

	"github.com/code-metrics/cli/internal/config"
	"github.com/code-metrics/cli/internal/git"
	"github.com/code-metrics/cli/internal/rules"
	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

func Run(opts *models.AnalyzerOptions) {
	if !git.CommitExists(opts.RepoPath, opts.DiffCommit1) {
		fmt.Fprintf(os.Stderr, "错误: Commit不存在: %s\n", opts.DiffCommit1)
		os.Exit(1)
	}
	if !git.CommitExists(opts.RepoPath, opts.DiffCommit2) {
		fmt.Fprintf(os.Stderr, "错误: Commit不存在: %s\n", opts.DiffCommit2)
		os.Exit(1)
	}

	startTime := time.Now()
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "开始增量分析: %s\n", utils.FormatFilepath(opts.RepoPath))
		fmt.Fprintf(os.Stderr, "对比范围: %s .. %s\n", opts.DiffCommit1, opts.DiffCommit2)
		fmt.Fprintf(os.Stderr, "并发数: %d\n\n", opts.Jobs)
	}

	configPath := opts.ConfigFile
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 配置文件加载失败: %v\n", err)
	}

	rulesEngine, err := rules.NewEngine(opts.RulesFile, opts.RepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 规则文件解析失败: %v\n", err)
		os.Exit(1)
	}

	report := &models.IncrementalReport{
		RepoPath:    opts.RepoPath,
		GeneratedAt: time.Now(),
		Commit1:     opts.DiffCommit1,
		Commit2:     opts.DiffCommit2,
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[1/4] 获取变更文件列表... ")
	}
	changedFiles, err := git.GetDiffFiles(opts.RepoPath, opts.DiffCommit1, opts.DiffCommit2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	report.ChangedFiles = filterSourceFiles(changedFiles)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成 (发现 %d 个变更文件)\n", len(report.ChangedFiles))
	}

	if len(report.ChangedFiles) == 0 {
		if !opts.CIMode {
			fmt.Fprintf(os.Stderr, "警告: 没有检测到源代码文件变更\n")
		}
		if rulesEngine != nil && rulesEngine.IsEnabled() {
			report.CustomRules = rulesEngine.EvaluateIncremental(report)
		}
		writeOutput(report, opts, cfg)
		os.Exit(0)
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[2/4] 分析复杂度变化... ")
	}
	report.Complexity = AnalyzeComplexityDiff(report.ChangedFiles, opts)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[3/4] 检测增量重复代码... ")
	}
	report.Duplication = AnalyzeDuplicationDiff(report.ChangedFiles, opts)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "[4/4] 分析依赖变动... ")
	}
	report.Dependency = AnalyzeDependencyDiff(report.ChangedFiles, opts)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "完成\n")
	}

	duration := time.Since(startTime)
	if !opts.CIMode {
		fmt.Fprintf(os.Stderr, "\n增量分析完成, 耗时: %s\n\n", duration.Round(time.Millisecond))
	}

	exitCode := 0
	qualityGateFailed := false
	if opts.CIMode {
		passed, violations := CheckIncrementalGates(report, cfg.QualityGates)
		report.QualityGates = &models.IncrementalGateResult{
			Passed:     passed,
			Violations: violations,
		}
		if !passed {
			qualityGateFailed = true
		}
	}

	if rulesEngine != nil && rulesEngine.IsEnabled() {
		report.CustomRules = rulesEngine.EvaluateIncremental(report)
	}

	if report.CustomRules != nil && report.CustomRules.HasErrors {
		exitCode = 3
	} else if qualityGateFailed {
		exitCode = 2
	}

	writeOutput(report, opts, cfg)
	os.Exit(exitCode)
}

func filterSourceFiles(files []models.ChangedFile) []models.ChangedFile {
	var result []models.ChangedFile
	for _, f := range files {
		if f.ChangeType == "deleted" {
			continue
		}
		lang := utils.GetLanguageByExt(f.FilePath)
		if lang != utils.LangUnknown {
			result = append(result, f)
		}
	}
	return result
}

func CheckIncrementalGates(report *models.IncrementalReport, gates models.QualityGates) (bool, []string) {
	var violations []string
	passed := true

	if report.Complexity != nil && report.Complexity.TotalDiff > 5 {
		violations = append(violations,
			fmt.Sprintf("复杂度净增超标: %d > 5", report.Complexity.TotalDiff))
		passed = false
	}

	if report.Dependency != nil && report.Dependency.NewCycleCount > 0 {
		violations = append(violations,
			fmt.Sprintf("新增循环依赖: 检测到 %d 个新循环", report.Dependency.NewCycleCount))
		passed = false
	}

	return passed, violations
}
