package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/code-metrics/cli/internal/config"
	"github.com/code-metrics/cli/internal/dependency"
	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

func Write(report *models.Report, opts *models.AnalyzerOptions, cfg *models.Config) {
	switch opts.Format {
	case "json":
		writeJSON(report, opts)
	case "html":
		writeHTML(report, opts)
	case "csv":
		writeCSV(report, opts)
	default:
		writeTerminal(report, opts, cfg)
	}
}

func writeJSON(report *models.Report, opts *models.AnalyzerOptions) {
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

func writeCSV(report *models.Report, opts *models.AnalyzerOptions) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	writer.Write([]string{"=代码质量度量报告="})
	writer.Write([]string{"仓库路径", report.RepoPath})
	writer.Write([]string{"生成时间", report.GeneratedAt.Format("2006-01-02 15:04:05")})
	writer.Write([]string{})

	if report.TechDebt != nil {
		writer.Write([]string{"=技术债评分="})
		writer.Write([]string{"评分", fmt.Sprintf("%.1f", report.TechDebt.Score)})
		writer.Write([]string{"等级", report.TechDebt.Grade})
		writer.Write([]string{"复杂度维度", fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Complexity)})
		writer.Write([]string{"重复代码维度", fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Duplication)})
		writer.Write([]string{"依赖维度", fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Dependency)})
		writer.Write([]string{"热点维度", fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Hotspots)})
		writer.Write([]string{})
	}

	if report.Metrics != nil {
		writer.Write([]string{"=基本度量="})
		writer.Write([]string{"语言", "文件数", "代码行", "注释行", "空行", "注释占比(%)"})
		for _, lm := range report.Metrics.ByLanguage {
			writer.Write([]string{
				lm.Language,
				fmt.Sprintf("%d", lm.FileCount),
				fmt.Sprintf("%d", lm.CodeLines),
				fmt.Sprintf("%d", lm.CommentLines),
				fmt.Sprintf("%d", lm.BlankLines),
				fmt.Sprintf("%.1f", lm.CommentRatio),
			})
		}
		writer.Write([]string{})
	}

	if report.Complexity != nil {
		writer.Write([]string{"=Top20高复杂度函数="})
		writer.Write([]string{"排名", "文件", "函数", "复杂度", "等级"})
		for i, f := range report.Complexity.TopComplex {
			writer.Write([]string{
				fmt.Sprintf("%d", i+1),
				f.FilePath,
				f.FunctionName,
				fmt.Sprintf("%d", f.Complexity),
				f.Level,
			})
		}
		writer.Write([]string{})
	}

	if report.GitHotspots != nil {
		writer.Write([]string{"=Top20变更热点文件="})
		writer.Write([]string{"排名", "文件", "修改次数", "修改行数", "Churn值", "高复杂度"})
		for i, h := range report.GitHotspots.TopHotspots {
			hc := "否"
			if h.HighComplexity {
				hc = "是"
			}
			writer.Write([]string{
				fmt.Sprintf("%d", i+1),
				h.FilePath,
				fmt.Sprintf("%d", h.Modifications),
				fmt.Sprintf("%d", h.LinesChanged),
				fmt.Sprintf("%.2f", h.Churn),
				hc,
			})
		}
		writer.Write([]string{})
	}

	if report.Contributors != nil {
		writer.Write([]string{"=作者贡献="})
		writer.Write([]string{"排名", "作者", "邮箱", "新增行数", "提交次数", "活跃文件数", "贡献占比(%)"})
		for i, c := range report.Contributors.Contributors {
			writer.Write([]string{
				fmt.Sprintf("%d", i+1),
				c.Name,
				c.Email,
				fmt.Sprintf("%d", c.AddedLines),
				fmt.Sprintf("%d", c.CommitCount),
				fmt.Sprintf("%d", c.ActiveFiles),
				fmt.Sprintf("%.1f", c.Contribution),
			})
		}
		writer.Write([]string{"Bus Factor", fmt.Sprintf("%d", report.Contributors.BusFactor)})
		writer.Write([]string{})
	}

	writer.Flush()

	if opts.OutputFile != "" {
		err := os.WriteFile(opts.OutputFile, []byte(sb.String()), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "写入文件错误: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "报告已写入: %s\n", opts.OutputFile)
	} else {
		fmt.Print(sb.String())
	}
}

func writeHTML(report *models.Report, opts *models.AnalyzerOptions) {
	html := generateHTML(report)

	if opts.OutputFile != "" {
		err := os.WriteFile(opts.OutputFile, []byte(html), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "写入文件错误: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "报告已写入: %s\n", opts.OutputFile)

		dotPath := strings.TrimSuffix(opts.OutputFile, filepath.Ext(opts.OutputFile)) + ".dot"
		if report.Dependency != nil && len(report.Dependency.Edges) > 0 {
			dependency.ExportDOT(report.Dependency, dotPath)
			fmt.Fprintf(os.Stderr, "依赖图已写入: %s\n", dotPath)
		}
	} else {
		fmt.Print(html)
	}
}

func writeTerminal(report *models.Report, opts *models.AnalyzerOptions, cfg *models.Config) {
	printHeader(report)
	printTechDebt(report)
	printMetrics(report)
	printComplexity(report)
	printDuplication(report)
	printDependency(report)
	printGitHotspots(report)
	printContributors(report)

	if report.Trend != nil {
		printTrend(report)
	}

	passed, violations := config.CheckQualityGates(report, cfg.QualityGates)
	if report.QualityGates == nil {
		report.QualityGates = &models.QualityGateResult{
			Passed:     passed,
			Violations: violations,
		}
	}

	fmt.Println()
	if passed {
		fmt.Println(utils.Green("✓ 质量门通过"))
	} else {
		fmt.Println(utils.Red("✗ 质量门未通过:"))
		for _, v := range violations {
			fmt.Printf("  - %s\n", v)
		}
	}

	if report.CustomRules != nil && report.CustomRules.Enabled {
		printCustomRules(report.CustomRules)
	}

	if opts.OutputFile != "" {
		dotPath := strings.TrimSuffix(opts.OutputFile, filepath.Ext(opts.OutputFile)) + ".dot"
		if report.Dependency != nil && len(report.Dependency.Edges) > 0 {
			dependency.ExportDOT(report.Dependency, dotPath)
			fmt.Fprintf(os.Stderr, "依赖图已写入: %s\n", dotPath)
		}
	}
}

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

func printHeader(report *models.Report) {
	fmt.Println()
	fmt.Println(utils.Bold(utils.BrightCyan("╔══════════════════════════════════════════════════════════════╗")))
	fmt.Println(utils.Bold(utils.BrightCyan("║") + "     Git仓库代码质量度量与技术债评估报告                " + utils.Bold(utils.BrightCyan("║"))))
	fmt.Println(utils.Bold(utils.BrightCyan("╚══════════════════════════════════════════════════════════════╝")))
	fmt.Println()
	fmt.Printf("仓库路径: %s\n", utils.Cyan(report.RepoPath))
	fmt.Printf("生成时间: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
}

func printTechDebt(report *models.Report) {
	if report.TechDebt == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("技术债评估")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))

	score := report.TechDebt.Score
	grade := report.TechDebt.Grade
	gradeColor := report.TechDebt.GradeColor

	fmt.Println()
	fmt.Printf("综合评分: %s\n", utils.Bold(utils.C(fmt.Sprintf("%.1f / 100", score), gradeColor)))
	fmt.Printf("等级:     %s\n", utils.Bold(utils.C(grade, gradeColor)))

	gradeDesc := map[string]string{
		"A": "健康 - 代码质量良好",
		"B": "轻微 - 存在少量技术债",
		"C": "中等 - 需要关注技术债",
		"D": "严重 - 技术债较严重",
		"F": "危险 - 急需重构",
	}
	if desc, ok := gradeDesc[grade]; ok {
		fmt.Printf("说明:     %s\n", desc)
	}

	fmt.Println()
	fmt.Println("评分构成:")
	printBar("复杂度 (30%)", report.TechDebt.Breakdown.Complexity, utils.ColorBrightMagenta)
	printBar("重复代码 (25%)", report.TechDebt.Breakdown.Duplication, utils.ColorBrightYellow)
	printBar("依赖 (20%)", report.TechDebt.Breakdown.Dependency, utils.ColorBrightBlue)
	printBar("热点 (25%)", report.TechDebt.Breakdown.Hotspots, utils.ColorBrightCyan)
	fmt.Println()
}

func printBar(label string, value float64, color string) {
	width := 40
	filled := int(value / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	fmt.Printf("  %-18s %s %5.1f\n", label, utils.C(bar, color), value)
}

func printMetrics(report *models.Report) {
	if report.Metrics == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("基本度量")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	headers := []string{"语言", "文件数", "代码行", "注释行", "空行", "注释占比"}
	colWidths := []int{12, 8, 10, 10, 8, 12}

	headerRow := ""
	for i, h := range headers {
		headerRow += utils.Bold(utils.PadCenter(h, colWidths[i]))
	}
	fmt.Println(utils.BrightCyan(headerRow))
	fmt.Println(utils.BrightCyan(strings.Repeat("─", sum(colWidths))))

	for _, lm := range report.Metrics.ByLanguage {
		row := fmt.Sprintf("%s%s%s%s%s%s",
			utils.PadRight(lm.Language, colWidths[0]),
			utils.PadLeft(fmt.Sprintf("%d", lm.FileCount), colWidths[1]),
			utils.PadLeft(fmt.Sprintf("%d", lm.CodeLines), colWidths[2]),
			utils.PadLeft(fmt.Sprintf("%d", lm.CommentLines), colWidths[3]),
			utils.PadLeft(fmt.Sprintf("%d", lm.BlankLines), colWidths[4]),
			utils.PadLeft(fmt.Sprintf("%.1f%%", lm.CommentRatio), colWidths[5]),
		)
		fmt.Println(row)
	}

	fmt.Println(utils.BrightCyan(strings.Repeat("─", sum(colWidths))))
	total := report.Metrics.Total
	totalRow := fmt.Sprintf("%s%s%s%s%s%s",
		utils.PadRight(utils.Bold("合计"), colWidths[0]),
		utils.PadLeft(utils.Bold(fmt.Sprintf("%d", total.FileCount)), colWidths[1]),
		utils.PadLeft(utils.Bold(fmt.Sprintf("%d", total.CodeLines)), colWidths[2]),
		utils.PadLeft(utils.Bold(fmt.Sprintf("%d", total.CommentLines)), colWidths[3]),
		utils.PadLeft(utils.Bold(fmt.Sprintf("%d", total.BlankLines)), colWidths[4]),
		utils.PadLeft(utils.Bold(fmt.Sprintf("%.1f%%", total.CommentRatio)), colWidths[5]),
	)
	fmt.Println(totalRow)
	fmt.Println()
}

func printComplexity(report *models.Report) {
	if report.Complexity == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("圈复杂度分析")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	fmt.Printf("平均圈复杂度: %s\n", utils.Bold(fmt.Sprintf("%.2f", report.Complexity.Average)))
	fmt.Printf("极高风险函数数: %s (%s)\n",
		utils.Bold(fmt.Sprintf("%d", report.Complexity.HighRiskCount)),
		utils.FormatPercent(report.Complexity.HighRiskRatio))
	fmt.Println()

	fmt.Println("复杂度分布直方图:")
	dist := report.Complexity.Distribution
	maxCount := 0
	for _, b := range dist {
		if b.Count > maxCount {
			maxCount = b.Count
		}
	}

	barWidth := 50
	for _, b := range dist {
		filled := 0
		if maxCount > 0 {
			filled = int(float64(b.Count) / float64(maxCount) * float64(barWidth))
		}
		bar := strings.Repeat("█", filled)
		var color string
		switch b.Label {
		case "简单":
			color = utils.ColorBrightGreen
		case "中等":
			color = utils.ColorBrightYellow
		case "复杂":
			color = utils.ColorBrightMagenta
		default:
			color = utils.ColorBrightRed
		}
		fmt.Printf("  %-8s %5d  %s\n",
			b.Range, b.Count, utils.C(bar, color))
	}
	fmt.Println()

	fmt.Println(utils.Bold("Top 20 高复杂度函数:"))
	fmt.Println()
	headers := []string{"排名", "复杂度", "等级", "函数", "文件"}
	colWidths := []int{6, 8, 8, 30, 40}

	headerRow := ""
	for i, h := range headers {
		headerRow += utils.Bold(utils.PadCenter(h, colWidths[i]))
	}
	fmt.Println(utils.BrightCyan(headerRow))
	fmt.Println(utils.BrightCyan(strings.Repeat("─", sum(colWidths))))

	for i, f := range report.Complexity.TopComplex {
		color := utils.ComplexityColor(f.Complexity)
		level := utils.C(f.Level, color)
		complexity := utils.C(fmt.Sprintf("%d", f.Complexity), color)

		funcName := f.FunctionName
		if len([]rune(funcName)) > colWidths[3]-2 {
			funcName = string([]rune(funcName)[:colWidths[3]-5]) + "..."
		}

		filePath := f.FilePath
		if len([]rune(filePath)) > colWidths[4]-2 {
			filePath = "..." + string([]rune(filePath)[len([]rune(filePath))-colWidths[4]+5:])
		}

		row := fmt.Sprintf("%s%s%s%s%s",
			utils.PadCenter(fmt.Sprintf("%d", i+1), colWidths[0]),
			utils.PadCenter(complexity, colWidths[1]),
			utils.PadCenter(level, colWidths[2]),
			utils.PadRight(funcName, colWidths[3]),
			utils.PadRight(filePath, colWidths[4]),
		)
		fmt.Println(row)
	}
	fmt.Println()
}

func printDuplication(report *models.Report) {
	if report.Duplication == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("重复代码检测")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	dupRate := report.Duplication.DuplicationRate
	var rateColor string
	if dupRate < 10 {
		rateColor = utils.ColorBrightGreen
	} else if dupRate < 20 {
		rateColor = utils.ColorBrightYellow
	} else {
		rateColor = utils.ColorBrightRed
	}

	fmt.Printf("重复率:     %s\n", utils.Bold(utils.C(fmt.Sprintf("%.1f%%", dupRate), rateColor)))
	fmt.Printf("总Token数:  %d\n", report.Duplication.TotalTokens)
	fmt.Printf("重复Token:  %d\n", report.Duplication.DuplicateTokens)
	fmt.Printf("重复块数:   %d\n", report.Duplication.BlockCount)
	fmt.Println()

	if len(report.Duplication.TopDuplicates) > 0 {
		fmt.Println(utils.Bold("Top 10 最大重复块:"))
		fmt.Println()
		for i, b := range report.Duplication.TopDuplicates {
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

func printDependency(report *models.Report) {
	if report.Dependency == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("依赖分析")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	fmt.Printf("节点数:      %d\n", len(report.Dependency.Nodes))
	fmt.Printf("边数:        %d\n", len(report.Dependency.Edges))
	fmt.Printf("循环依赖:    %s\n", formatCount(report.Dependency.CycleCount))
	fmt.Printf("高耦合文件:  %s\n", formatCount(report.Dependency.HighCouplingCount))
	fmt.Println()

	if report.Dependency.CycleCount > 0 {
		fmt.Println(utils.Red("发现循环依赖:"))
		for i, cycle := range report.Dependency.Cycles {
			fmt.Printf("  环 %d: %s\n", i+1, strings.Join(cycle, " → "))
		}
		fmt.Println()
	}

	if len(report.Dependency.HighCoupling) > 0 {
		fmt.Println(utils.Yellow("高耦合文件 (扇出 > 15):"))
		for _, f := range report.Dependency.HighCoupling {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println()
	}

	if len(report.Dependency.FanInOut) > 0 {
		fmt.Println(utils.Bold("扇入/扇出 Top 10:"))
		fmt.Println()
		headers := []string{"文件", "扇入", "扇出"}
		colWidths := []int{50, 6, 6}

		headerRow := ""
		for i, h := range headers {
			headerRow += utils.Bold(utils.PadCenter(h, colWidths[i]))
		}
		fmt.Println(utils.BrightCyan(headerRow))
		fmt.Println(utils.BrightCyan(strings.Repeat("─", sum(colWidths))))

		count := 10
		if len(report.Dependency.FanInOut) < count {
			count = len(report.Dependency.FanInOut)
		}
		for _, fo := range report.Dependency.FanInOut[:count] {
			filePath := fo.File
			if len([]rune(filePath)) > colWidths[0]-2 {
				filePath = "..." + string([]rune(filePath)[len([]rune(filePath))-colWidths[0]+5:])
			}
			fanOutColor := utils.ColorWhite
			if fo.FanOut > 15 {
				fanOutColor = utils.ColorBrightRed
			}
			row := fmt.Sprintf("%s%s%s",
				utils.PadRight(filePath, colWidths[0]),
				utils.PadLeft(fmt.Sprintf("%d", fo.FanIn), colWidths[1]),
				utils.PadLeft(utils.C(fmt.Sprintf("%d", fo.FanOut), fanOutColor), colWidths[2]),
			)
			fmt.Println(row)
		}
		fmt.Println()
	}
}

func printGitHotspots(report *models.Report) {
	if report.GitHotspots == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("Git变更热点")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	fmt.Printf("时间范围: %s\n", report.GitHotspots.TimeRange)
	fmt.Println()

	if len(report.GitHotspots.HighPriorityFiles) > 0 {
		fmt.Println(utils.BrightRed("重构优先级最高 (高复杂度 + 高变更):"))
		for _, f := range report.GitHotspots.HighPriorityFiles {
			fmt.Printf("  ⚡ %s\n", f)
		}
		fmt.Println()
	}

	fmt.Println(utils.Bold("Top 20 变更热点文件:"))
	fmt.Println()
	headers := []string{"排名", "修改次数", "修改行数", "Churn值", "复杂度", "文件"}
	colWidths := []int{6, 10, 10, 10, 8, 46}

	headerRow := ""
	for i, h := range headers {
		headerRow += utils.Bold(utils.PadCenter(h, colWidths[i]))
	}
	fmt.Println(utils.BrightCyan(headerRow))
	fmt.Println(utils.BrightCyan(strings.Repeat("─", sum(colWidths))))

	for i, h := range report.GitHotspots.TopHotspots {
		hc := "  "
		hcColor := utils.ColorWhite
		if h.HighComplexity {
			hc = "高"
			hcColor = utils.ColorBrightRed
		}

		filePath := h.FilePath
		if len([]rune(filePath)) > colWidths[5]-2 {
			filePath = "..." + string([]rune(filePath)[len([]rune(filePath))-colWidths[5]+5:])
		}

		row := fmt.Sprintf("%s%s%s%s%s%s",
			utils.PadCenter(fmt.Sprintf("%d", i+1), colWidths[0]),
			utils.PadLeft(fmt.Sprintf("%d", h.Modifications), colWidths[1]),
			utils.PadLeft(fmt.Sprintf("%d", h.LinesChanged), colWidths[2]),
			utils.PadLeft(fmt.Sprintf("%.2f", h.Churn), colWidths[3]),
			utils.PadCenter(utils.C(hc, hcColor), colWidths[4]),
			utils.PadRight(filePath, colWidths[5]),
		)
		fmt.Println(row)
	}
	fmt.Println()
}

func printContributors(report *models.Report) {
	if report.Contributors == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("作者贡献分析")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	busFactor := report.Contributors.BusFactor
	busFactorStr := fmt.Sprintf("%d", busFactor)
	if busFactor == 1 {
		busFactorStr = utils.Red(busFactorStr + " ⚠️  单点人员风险!")
	} else if busFactor <= 2 {
		busFactorStr = utils.Yellow(busFactorStr)
	} else {
		busFactorStr = utils.Green(busFactorStr)
	}

	fmt.Printf("Bus Factor: %s\n", busFactorStr)
	fmt.Printf("总新增行数: %d\n", report.Contributors.TotalLines)
	fmt.Println()

	if len(report.Contributors.Contributors) > 0 {
		fmt.Println(utils.Bold("贡献排行:"))
		fmt.Println()
		headers := []string{"排名", "作者", "新增行数", "提交次数", "活跃文件", "贡献占比"}
		colWidths := []int{6, 20, 10, 10, 10, 12}

		headerRow := ""
		for i, h := range headers {
			headerRow += utils.Bold(utils.PadCenter(h, colWidths[i]))
		}
		fmt.Println(utils.BrightCyan(headerRow))
		fmt.Println(utils.BrightCyan(strings.Repeat("─", sum(colWidths))))

		count := 10
		if len(report.Contributors.Contributors) < count {
			count = len(report.Contributors.Contributors)
		}
		for i, c := range report.Contributors.Contributors[:count] {
			name := c.Name
			if len([]rune(name)) > colWidths[1]-2 {
				name = string([]rune(name)[:colWidths[1]-5]) + "..."
			}

			contribBarWidth := 20
			contribFilled := int(c.Contribution / 100 * float64(contribBarWidth))
			contribBar := strings.Repeat("█", contribFilled) + strings.Repeat("░", contribBarWidth-contribFilled)

			row := fmt.Sprintf("%s%s%s%s%s%s %s",
				utils.PadCenter(fmt.Sprintf("%d", i+1), colWidths[0]),
				utils.PadRight(name, colWidths[1]),
				utils.PadLeft(fmt.Sprintf("%d", c.AddedLines), colWidths[2]),
				utils.PadLeft(fmt.Sprintf("%d", c.CommitCount), colWidths[3]),
				utils.PadLeft(fmt.Sprintf("%d", c.ActiveFiles), colWidths[4]),
				utils.PadLeft(fmt.Sprintf("%.1f%%", c.Contribution), colWidths[5]),
				utils.C(contribBar, utils.ColorBrightCyan),
			)
			fmt.Println(row)
		}
		fmt.Println()
	}
}

func printTrend(report *models.Report) {
	if report.Trend == nil {
		return
	}

	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println(utils.Bold(utils.BrightYellow("趋势分析")))
	fmt.Println(utils.Bold(utils.BrightYellow("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")))
	fmt.Println()

	headers := []string{"月份", "代码行数", "代码增长", "平均复杂度", "复杂度变化", "重复率", "重复率变化"}
	colWidths := []int{10, 12, 12, 14, 14, 10, 14}

	headerRow := ""
	for i, h := range headers {
		headerRow += utils.Bold(utils.PadCenter(h, colWidths[i]))
	}
	fmt.Println(utils.BrightCyan(headerRow))
	fmt.Println(utils.BrightCyan(strings.Repeat("─", sum(colWidths))))

	for _, m := range report.Trend.Months {
		if m.CodeLines == 0 {
			continue
		}

		growthStr := formatGrowth(m.Growth)
		complexityGrowthStr := formatGrowth(m.ComplexityGrowth)
		dupGrowthStr := formatGrowth(m.DuplicationGrowth)

		row := fmt.Sprintf("%s%s%s%s%s%s%s",
			utils.PadCenter(m.Month, colWidths[0]),
			utils.PadLeft(fmt.Sprintf("%d", m.CodeLines), colWidths[1]),
			utils.PadLeft(growthStr, colWidths[2]),
			utils.PadLeft(fmt.Sprintf("%.2f", m.AvgComplexity), colWidths[3]),
			utils.PadLeft(complexityGrowthStr, colWidths[4]),
			utils.PadLeft(fmt.Sprintf("%.1f%%", m.DuplicationRate), colWidths[5]),
			utils.PadLeft(dupGrowthStr, colWidths[6]),
		)
		fmt.Println(row)
	}
	fmt.Println()
}

func formatGrowth(v float64) string {
	if v == 0 {
		return "-"
	}
	str := fmt.Sprintf("%+.1f%%", v)
	if v > 10 {
		return utils.Red(str + " ↑")
	} else if v > 0 {
		return utils.Yellow(str)
	} else if v < -10 {
		return utils.Green(str + " ↓")
	}
	return str
}

func formatCount(n int) string {
	if n == 0 {
		return utils.Green("0 (无)")
	}
	return utils.Red(fmt.Sprintf("%d", n))
}

func sum(nums []int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}
