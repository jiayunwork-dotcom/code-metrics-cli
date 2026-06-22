package incremental

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

func printCustomRules(custom *models.CustomRulesResult) {
	fmt.Println()
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("自定义规则评估")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	if custom.RulesFile != "" {
		fmt.Printf("规则文件: %s\n", utils.Cyan(custom.RulesFile))
		fmt.Println()
	}

	fmt.Printf("总规则数: %d, 通过: %s, 不通过: %s, 跳过: %s\n\n",
		custom.TotalRules,
		utils.Green(fmt.Sprintf("%d", custom.PassedCount)),
		utils.Red(fmt.Sprintf("%d", custom.FailedCount)),
		utils.Yellow(fmt.Sprintf("%d", custom.SkippedCount)),
	)

	for _, group := range custom.Groups {
		groupStatus := utils.Green("✓")
		if !group.Passed {
			groupStatus = utils.Red("✗")
		}
		logicStr := "AND"
		if group.Logic == models.LogicOR {
			logicStr = "OR"
		}
		fmt.Printf("%s %s [%s]\n", groupStatus, utils.Bold(group.GroupName), logicStr)

		for _, r := range group.Results {
			var statusStr string
			var colorFunc func(string) string

			switch r.Status {
			case models.RuleStatusPassed:
				statusStr = "通过"
				colorFunc = utils.Green
			case models.RuleStatusFailed:
				statusStr = "不通过"
				colorFunc = utils.Red
			case models.RuleStatusSkipped:
				statusStr = "跳过"
				colorFunc = utils.Yellow
			}

			severityStr := ""
			switch r.Severity {
			case models.SeverityError:
				severityStr = " [error]"
			case models.SeverityWarning:
				severityStr = " [warning]"
			case models.SeverityInfo:
				severityStr = " [info]"
			}

			fmt.Printf("  %s %s%s - %s",
				colorFunc("●"),
				r.RuleName,
				severityStr,
				statusStr,
			)

			if r.Actual != "" {
				fmt.Printf(" (实际值: %s)", r.Actual)
			}

			if r.Message != "" && r.Status == models.RuleStatusFailed {
				fmt.Printf(" - %s", r.Message)
			}

			if r.SkipReason != "" {
				fmt.Printf(" (%s)", r.SkipReason)
			}
			fmt.Println()
		}
		fmt.Println()
	}
}

func writeOutput(report *models.IncrementalReport, opts *models.AnalyzerOptions, cfg *models.Config) {
	switch opts.Format {
	case "json":
		writeIncrementalJSON(report, opts)
	default:
		writeIncrementalTerminal(report, opts, cfg)
	}
}

func writeIncrementalJSON(report *models.IncrementalReport, opts *models.AnalyzerOptions) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON序列化错误: %v\n", err)
		return
	}

	if opts.OutputFile != "" {
		err = os.WriteFile(opts.OutputFile, data, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "写入文件错误: %v\n", err)
			return
		}
		if !opts.CIMode {
			fmt.Fprintf(os.Stderr, "报告已写入: %s\n", opts.OutputFile)
		}
	} else {
		fmt.Println(string(data))
	}
}

func writeIncrementalTerminal(report *models.IncrementalReport, opts *models.AnalyzerOptions, cfg *models.Config) {
	printIncrementalHeader(report)
	printChangedFiles(report)
	printComplexityDiff(report)
	printDuplicationDiff(report)
	printDependencyDiff(report)

	if opts.CIMode && report.QualityGates != nil {
		fmt.Println()
		if report.QualityGates.Passed {
			fmt.Println(utils.Green("✓ 增量质量门通过"))
		} else {
			fmt.Println(utils.Red("✗ 增量质量门未通过:"))
			for _, v := range report.QualityGates.Violations {
				fmt.Printf("  - %s\n", v)
			}
		}
	}

	if report.CustomRules != nil && report.CustomRules.Enabled {
		printCustomRules(report.CustomRules)
	}
}

func printIncrementalHeader(report *models.IncrementalReport) {
	fmt.Println()
	fmt.Println(utils.Bold(utils.BrightCyan("╔══════════════════════════════════════════════════════════════╗")))
	fmt.Println(utils.Bold(utils.BrightCyan("║") + "               增量代码质量分析报告                        " + utils.Bold(utils.BrightCyan("║"))))
	fmt.Println(utils.Bold(utils.BrightCyan("╚══════════════════════════════════════════════════════════════╝")))
	fmt.Println()
	fmt.Printf("仓库路径: %s\n", utils.Cyan(report.RepoPath))
	fmt.Printf("对比范围: %s → %s\n", utils.Yellow(report.Commit1), utils.Green(report.Commit2))
	fmt.Printf("生成时间: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
}

func printChangedFiles(report *models.IncrementalReport) {
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("变更文件列表")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	if len(report.ChangedFiles) == 0 {
		fmt.Println("  没有检测到源代码文件变更")
		fmt.Println()
		return
	}

	typeCount := make(map[string]int)
	for _, f := range report.ChangedFiles {
		typeCount[f.ChangeType]++
	}

	fmt.Printf("总变更文件数: %s\n", utils.Bold(fmt.Sprintf("%d", len(report.ChangedFiles))))
	if typeCount["added"] > 0 {
		fmt.Printf("  新增文件: %s\n", utils.Green(fmt.Sprintf("%d", typeCount["added"])))
	}
	if typeCount["modified"] > 0 {
		fmt.Printf("  修改文件: %s\n", utils.Yellow(fmt.Sprintf("%d", typeCount["modified"])))
	}
	if typeCount["renamed"] > 0 {
		fmt.Printf("  重命名文件: %s\n", utils.Cyan(fmt.Sprintf("%d", typeCount["renamed"])))
	}
	if typeCount["copied"] > 0 {
		fmt.Printf("  复制文件: %s\n", utils.Cyan(fmt.Sprintf("%d", typeCount["copied"])))
	}
	fmt.Println()

	fmt.Println(utils.Bold("文件列表:"))
	for _, f := range report.ChangedFiles {
		var typeStr string
		switch f.ChangeType {
		case "added":
			typeStr = utils.Green("[新增]")
		case "modified":
			typeStr = utils.Yellow("[修改]")
		case "renamed":
			typeStr = utils.Cyan("[重命名]")
		case "copied":
			typeStr = utils.Cyan("[复制]")
		default:
			typeStr = utils.White("[变更]")
		}

		if f.OldPath != "" {
			fmt.Printf("  %s %s → %s\n", typeStr, f.OldPath, f.FilePath)
		} else {
			fmt.Printf("  %s %s\n", typeStr, f.FilePath)
		}
	}
	fmt.Println()
}

func printComplexityDiff(report *models.IncrementalReport) {
	if report.Complexity == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("圈复杂度变化")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	diff := report.Complexity.TotalDiff
	var diffStr string
	if diff > 0 {
		diffStr = utils.Red(fmt.Sprintf("+%d", diff))
	} else if diff < 0 {
		diffStr = utils.Green(fmt.Sprintf("%d", diff))
	} else {
		diffStr = utils.White(fmt.Sprintf("%d", diff))
	}

	fmt.Printf("复杂度总变化: %s\n", diffStr)
	fmt.Printf("分析文件数: %d\n", report.Complexity.ChangedFiles)
	if report.Complexity.ImprovedFiles > 0 {
		fmt.Printf("改善文件: %s\n", utils.Green(fmt.Sprintf("%d", report.Complexity.ImprovedFiles)))
	}
	if report.Complexity.DegradedFiles > 0 {
		fmt.Printf("恶化文件: %s\n", utils.Red(fmt.Sprintf("%d", report.Complexity.DegradedFiles)))
	}
	if report.Complexity.NewHighRiskCount > 0 {
		fmt.Printf("新增高风险函数: %s\n", utils.Red(fmt.Sprintf("%d", report.Complexity.NewHighRiskCount)))
	}
	fmt.Println()

	if len(report.Complexity.FileDiffs) > 0 {
		fmt.Println(utils.Bold("各文件复杂度变化:"))
		fmt.Println()
		headers := []string{"变化", "文件", "旧值", "新值", "差值"}
		colWidths := []int{8, 50, 8, 8, 8}

		headerRow := ""
		for i, h := range headers {
			headerRow += utils.Bold(utils.PadCenter(h, colWidths[i]))
		}
		fmt.Println(utils.BrightCyan(headerRow))
		fmt.Println(utils.BrightCyan(strings.Repeat("─", sumInc(colWidths))))

		for _, fd := range report.Complexity.FileDiffs {
			var diffSign string
			var diffColor string
			if fd.Diff > 0 {
				diffSign = fmt.Sprintf("+%d", fd.Diff)
				diffColor = utils.ColorBrightRed
			} else if fd.Diff < 0 {
				diffSign = fmt.Sprintf("%d", fd.Diff)
				diffColor = utils.ColorBrightGreen
			} else {
				diffSign = fmt.Sprintf("%d", fd.Diff)
				diffColor = utils.ColorBrightWhite
			}

			var status string
			if fd.Diff > 0 {
				status = utils.Red("↑")
			} else if fd.Diff < 0 {
				status = utils.Green("↓")
			} else {
				status = utils.White("=")
			}

			filePath := fd.FilePath
			if len([]rune(filePath)) > colWidths[1]-2 {
				filePath = "..." + string([]rune(filePath)[len([]rune(filePath))-colWidths[1]+5:])
			}

			row := fmt.Sprintf("%s%s%s%s%s",
				utils.PadCenter(status, colWidths[0]),
				utils.PadRight(filePath, colWidths[1]),
				utils.PadLeft(fmt.Sprintf("%d", fd.OldComplexity), colWidths[2]),
				utils.PadLeft(fmt.Sprintf("%d", fd.NewComplexity), colWidths[3]),
				utils.PadLeft(utils.C(diffSign, diffColor), colWidths[4]),
			)
			fmt.Println(row)
		}
		fmt.Println(utils.BrightCyan(strings.Repeat("─", sumInc(colWidths))))
		fmt.Println()
	}

	if report.Complexity.NewHighRiskCount > 0 {
		fmt.Println(utils.Red(utils.Bold("新增高风险函数 (复杂度 > 20):")))
		fmt.Println()
		for _, fd := range report.Complexity.FileDiffs {
			if len(fd.NewHighRisk) > 0 {
				for _, f := range fd.NewHighRisk {
					color := utils.ComplexityColor(f.Complexity)
					fmt.Printf("  %s - %s (%s)\n",
						utils.C(fmt.Sprintf("%d", f.Complexity), color),
						f.FunctionName,
						f.FilePath)
				}
			}
		}
		fmt.Println()
	}
}

func printDuplicationDiff(report *models.IncrementalReport) {
	if report.Duplication == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("增量重复代码检测")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	dupRate := report.Duplication.NewDuplicationRate
	var rateColor string
	if dupRate < 10 {
		rateColor = utils.ColorBrightGreen
	} else if dupRate < 20 {
		rateColor = utils.ColorBrightYellow
	} else {
		rateColor = utils.ColorBrightRed
	}

	fmt.Printf("新增重复率: %s\n", utils.Bold(utils.C(fmt.Sprintf("%.1f%%", dupRate), rateColor)))
	fmt.Printf("新增重复Token: %d / %d\n", report.Duplication.NewDuplicateTokens, report.Duplication.NewTotalTokens)
	fmt.Printf("新增重复块数: %d\n", report.Duplication.NewBlockCount)
	fmt.Println()

	if len(report.Duplication.NewTopDuplicates) > 0 {
		fmt.Println(utils.Bold("新增重复块详情:"))
		fmt.Println()
		for i, b := range report.Duplication.NewTopDuplicates {
			crossFile := ""
			if b.IsCrossFile {
				crossFile = utils.Red(" [跨文件]")
			}
			fmt.Printf("%2d. Token长度: %d, 出现次数: %d%s\n",
				i+1, b.TokenLength, b.Occurrences, crossFile)
			for _, loc := range b.Locations {
				fmt.Printf("    %s:%d-%d\n", loc.FilePath, loc.StartLine, loc.EndLine)
			}
			fmt.Println()
		}
	}
}

func printDependencyDiff(report *models.IncrementalReport) {
	if report.Dependency == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("依赖变动分析")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	if len(report.Dependency.AddedEdges) > 0 {
		fmt.Println(utils.Yellow("新增依赖边:"))
		for _, e := range report.Dependency.AddedEdges {
			fmt.Printf("  %s %s %s\n", e.From, utils.Green("→"), e.To)
		}
		fmt.Println()
	}

	if len(report.Dependency.RemovedEdges) > 0 {
		fmt.Println(utils.Green("删除依赖边:"))
		for _, e := range report.Dependency.RemovedEdges {
			fmt.Printf("  %s %s %s\n", e.From, utils.Red("→"), e.To)
		}
		fmt.Println()
	}

	if report.Dependency.NewCycleCount > 0 {
		fmt.Println(utils.Red(utils.Bold("⚠️  新增循环依赖:")))
		for i, cycle := range report.Dependency.NewCycles {
			fmt.Printf("  环 %d: %s\n", i+1, strings.Join(cycle, " → "))
		}
		fmt.Println()
	}

	if len(report.Dependency.AddedEdges) == 0 &&
		len(report.Dependency.RemovedEdges) == 0 &&
		report.Dependency.NewCycleCount == 0 {
		fmt.Println("  没有检测到依赖变动")
		fmt.Println()
	}
}

func sumInc(nums []int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}
