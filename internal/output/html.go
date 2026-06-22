package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/code-metrics/cli/pkg/models"
)

func generateHTML(report *models.Report) string {
	jsonData, _ := json.MarshalIndent(report, "", "  ")

	gradeColor := map[string]string{
		"A": "#10b981",
		"B": "#06b6d4",
		"C": "#eab308",
		"D": "#d946ef",
		"F": "#ef4444",
	}

	grade := report.TechDebt.Grade
	color := gradeColor[grade]
	if color == "" {
		color = "#6b7280"
	}

	var html strings.Builder

	html.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>代码质量度量报告 - ` + report.RepoPath + `</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        .header {
            background: white;
            border-radius: 16px;
            padding: 30px;
            margin-bottom: 20px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
        }
        .header h1 {
            font-size: 28px;
            color: #1f2937;
            margin-bottom: 10px;
        }
        .header .meta {
            color: #6b7280;
            font-size: 14px;
        }
        .score-card {
            background: white;
            border-radius: 16px;
            padding: 30px;
            margin-bottom: 20px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
            text-align: center;
        }
        .score-value {
            font-size: 72px;
            font-weight: bold;
            color: ` + color + `;
            margin: 20px 0;
        }
        .grade-badge {
            display: inline-block;
            padding: 8px 24px;
            border-radius: 50px;
            background: ` + color + `;
            color: white;
            font-size: 24px;
            font-weight: bold;
            margin-bottom: 20px;
        }
        .score-desc {
            color: #6b7280;
            font-size: 16px;
            margin-bottom: 20px;
        }
        .score-breakdown {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-top: 20px;
        }
        .breakdown-item {
            background: #f3f4f6;
            padding: 15px;
            border-radius: 10px;
            text-align: left;
        }
        .breakdown-label {
            font-size: 14px;
            color: #6b7280;
            margin-bottom: 8px;
        }
        .breakdown-bar {
            height: 8px;
            background: #e5e7eb;
            border-radius: 4px;
            overflow: hidden;
        }
        .breakdown-fill {
            height: 100%;
            border-radius: 4px;
            transition: width 0.5s;
        }
        .section {
            background: white;
            border-radius: 16px;
            padding: 24px;
            margin-bottom: 20px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
        }
        .section-header {
            cursor: pointer;
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding-bottom: 15px;
            border-bottom: 2px solid #e5e7eb;
            user-select: none;
        }
        .section-header h2 {
            font-size: 20px;
            color: #1f2937;
        }
        .section-toggle {
            font-size: 24px;
            color: #6b7280;
            transition: transform 0.3s;
        }
        .section.open .section-toggle {
            transform: rotate(180deg);
        }
        .section-content {
            max-height: 0;
            overflow: hidden;
            transition: max-height 0.5s ease-out;
        }
        .section.open .section-content {
            max-height: 5000px;
            padding-top: 20px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 15px;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #e5e7eb;
        }
        th {
            background: #f3f4f6;
            font-weight: 600;
            color: #374151;
        }
        tr:hover {
            background: #f9fafb;
        }
        .badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: 600;
        }
        .badge-simple { background: #d1fae5; color: #065f46; }
        .badge-medium { background: #fef3c7; color: #92400e; }
        .badge-complex { background: #fae8ff; color: #86198f; }
        .badge-danger { background: #fee2e2; color: #991b1b; }
        .badge-crossfile { background: #fef2f2; color: #dc2626; }
        .highlight {
            background: #fef3c7;
            padding: 2px 6px;
            border-radius: 4px;
        }
        .high-priority {
            background: #fee2e2;
            padding: 10px 15px;
            border-radius: 8px;
            margin: 10px 0;
            border-left: 4px solid #ef4444;
        }
        .bus-factor-1 {
            background: #fee2e2;
            padding: 15px;
            border-radius: 8px;
            color: #991b1b;
            font-weight: 600;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
        }
        .stat-item {
            background: #f3f4f6;
            padding: 15px;
            border-radius: 10px;
            text-align: center;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #1f2937;
        }
        .stat-label {
            font-size: 12px;
            color: #6b7280;
            margin-top: 5px;
        }
        .chart-container {
            margin: 20px 0;
        }
        .bar-chart {
            display: flex;
            align-items: flex-end;
            height: 200px;
            gap: 10px;
            padding: 20px;
            background: #f9fafb;
            border-radius: 10px;
        }
        .bar {
            flex: 1;
            display: flex;
            flex-direction: column;
            align-items: center;
        }
        .bar-fill {
            width: 100%;
            border-radius: 4px 4px 0 0;
            min-height: 2px;
            transition: height 0.5s;
        }
        .bar-label {
            margin-top: 8px;
            font-size: 11px;
            color: #6b7280;
            text-align: center;
        }
        .bar-count {
            font-size: 12px;
            font-weight: 600;
            color: #374151;
            margin-bottom: 4px;
        }
        code {
            background: #f3f4f6;
            padding: 2px 6px;
            border-radius: 4px;
            font-family: 'Monaco', 'Consolas', monospace;
            font-size: 13px;
        }
        pre {
            background: #1f2937;
            color: #e5e7eb;
            padding: 15px;
            border-radius: 8px;
            overflow-x: auto;
            margin-top: 15px;
        }
        .trend-up { color: #ef4444; }
        .trend-down { color: #10b981; }
        .cycle-list {
            background: #fef2f2;
            padding: 15px;
            border-radius: 8px;
            margin: 10px 0;
        }
        .cycle-item {
            padding: 8px 0;
            border-bottom: 1px solid #fecaca;
        }
        .cycle-item:last-child {
            border-bottom: none;
        }
        @media (max-width: 768px) {
            .score-value { font-size: 48px; }
            table { font-size: 12px; }
            th, td { padding: 8px; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📊 Git仓库代码质量度量与技术债评估报告</h1>
            <div class="meta">
                <p><strong>仓库路径:</strong> ` + report.RepoPath + `</p>
                <p><strong>生成时间:</strong> ` + report.GeneratedAt.Format("2006-01-02 15:04:05") + `</p>
            </div>
        </div>

        ` + generateTechDebtHTML(report, color) + `
        ` + generateMetricsHTML(report) + `
        ` + generateComplexityHTML(report) + `
        ` + generateDuplicationHTML(report) + `
        ` + generateDependencyHTML(report) + `
        ` + generateHotspotsHTML(report) + `
        ` + generateContributorsHTML(report) + `
        ` + generateTrendHTML(report) + `

        <div class="section">
            <div class="section-header" onclick="toggleSection(this)">
                <h2>📋 原始数据 (JSON)</h2>
                <span class="section-toggle">▼</span>
            </div>
            <div class="section-content">
                <pre>` + escapeHTML(string(jsonData)) + `</pre>
            </div>
        </div>
    </div>

    <script>
        function toggleSection(header) {
            const section = header.parentElement;
            section.classList.toggle('open');
        }

        document.addEventListener('DOMContentLoaded', function() {
            const firstSection = document.querySelector('.section');
            if (firstSection) {
                firstSection.classList.add('open');
            }

            const sections = document.querySelectorAll('.section');
            sections.forEach(section => {
                const content = section.querySelector('.section-content');
                if (section.classList.contains('open')) {
                    content.style.maxHeight = content.scrollHeight + 'px';
                }
            });
        });
    </script>
</body>
</html>`)

	return html.String()
}

func generateTechDebtHTML(report *models.Report, color string) string {
	if report.TechDebt == nil {
		return ""
	}

	gradeDesc := map[string]string{
		"A": "健康 - 代码质量良好",
		"B": "轻微 - 存在少量技术债",
		"C": "中等 - 需要关注技术债",
		"D": "严重 - 技术债较严重",
		"F": "危险 - 急需重构",
	}

	return `
        <div class="score-card">
            <div class="grade-badge">` + report.TechDebt.Grade + `级</div>
            <div class="score-value">` + fmt.Sprintf("%.1f", report.TechDebt.Score) + `</div>
            <div class="score-desc">` + gradeDesc[report.TechDebt.Grade] + `</div>
            <div class="score-breakdown">
                <div class="breakdown-item">
                    <div class="breakdown-label">复杂度 (30%) - ` + fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Complexity) + `</div>
                    <div class="breakdown-bar"><div class="breakdown-fill" style="width: ` + fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Complexity) + `%; background: #d946ef;"></div></div>
                </div>
                <div class="breakdown-item">
                    <div class="breakdown-label">重复代码 (25%) - ` + fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Duplication) + `</div>
                    <div class="breakdown-bar"><div class="breakdown-fill" style="width: ` + fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Duplication) + `%; background: #eab308;"></div></div>
                </div>
                <div class="breakdown-item">
                    <div class="breakdown-label">依赖 (20%) - ` + fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Dependency) + `</div>
                    <div class="breakdown-bar"><div class="breakdown-fill" style="width: ` + fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Dependency) + `%; background: #3b82f6;"></div></div>
                </div>
                <div class="breakdown-item">
                    <div class="breakdown-label">热点 (25%) - ` + fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Hotspots) + `</div>
                    <div class="breakdown-bar"><div class="breakdown-fill" style="width: ` + fmt.Sprintf("%.1f", report.TechDebt.Breakdown.Hotspots) + `%; background: #06b6d4;"></div></div>
                </div>
            </div>
        </div>`
}

func generateMetricsHTML(report *models.Report) string {
	if report.Metrics == nil {
		return ""
	}

	var rows strings.Builder
	for _, lm := range report.Metrics.ByLanguage {
		rows.WriteString(`
                <tr>
                    <td>` + lm.Language + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", lm.FileCount) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", lm.CodeLines) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", lm.CommentLines) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", lm.BlankLines) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%.1f%%", lm.CommentRatio) + `</td>
                </tr>`)
	}

	total := report.Metrics.Total
	rows.WriteString(`
                <tr style="font-weight: bold; background: #f3f4f6;">
                    <td>合计</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", total.FileCount) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", total.CodeLines) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", total.CommentLines) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", total.BlankLines) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%.1f%%", total.CommentRatio) + `</td>
                </tr>`)

	return `
        <div class="section open">
            <div class="section-header" onclick="toggleSection(this)">
                <h2>📈 基本度量</h2>
                <span class="section-toggle">▼</span>
            </div>
            <div class="section-content">
                <table>
                    <thead>
                        <tr>
                            <th>语言</th>
                            <th style="text-align: right;">文件数</th>
                            <th style="text-align: right;">代码行</th>
                            <th style="text-align: right;">注释行</th>
                            <th style="text-align: right;">空行</th>
                            <th style="text-align: right;">注释占比</th>
                        </tr>
                    </thead>
                    <tbody>` + rows.String() + `
                    </tbody>
                </table>
            </div>
        </div>`
}

func generateComplexityHTML(report *models.Report) string {
	if report.Complexity == nil {
		return ""
	}

	badgeClass := map[string]string{
		"简单":   "badge-simple",
		"中等":   "badge-medium",
		"复杂":   "badge-complex",
		"极高风险": "badge-danger",
	}

	barColor := map[string]string{
		"简单":   "#10b981",
		"中等":   "#eab308",
		"复杂":   "#d946ef",
		"极高风险": "#ef4444",
	}

	maxCount := 0
	for _, b := range report.Complexity.Distribution {
		if b.Count > maxCount {
			maxCount = b.Count
		}
	}

	var bars strings.Builder
	for _, b := range report.Complexity.Distribution {
		height := 0
		if maxCount > 0 {
			height = int(float64(b.Count) / float64(maxCount) * 150)
		}
		bars.WriteString(`
                    <div class="bar">
                        <div class="bar-count">` + fmt.Sprintf("%d", b.Count) + `</div>
                        <div class="bar-fill" style="height: ` + fmt.Sprintf("%d", height) + `px; background: ` + barColor[b.Label] + `;"></div>
                        <div class="bar-label">` + b.Range + `<br>` + b.Label + `</div>
                    </div>`)
	}

	var funcRows strings.Builder
	for i, f := range report.Complexity.TopComplex {
		funcRows.WriteString(`
                <tr>
                    <td style="text-align: center;">` + fmt.Sprintf("%d", i+1) + `</td>
                    <td style="text-align: right; font-weight: bold;">` + fmt.Sprintf("%d", f.Complexity) + `</td>
                    <td><span class="badge ` + badgeClass[f.Level] + `">` + f.Level + `</span></td>
                    <td><code>` + f.FunctionName + `</code></td>
                    <td>` + f.FilePath + `</td>
                </tr>`)
	}

	return `
        <div class="section">
            <div class="section-header" onclick="toggleSection(this)">
                <h2>🔄 圈复杂度分析</h2>
                <span class="section-toggle">▼</span>
            </div>
            <div class="section-content">
                <div class="stats-grid">
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%.2f", report.Complexity.Average) + `</div>
                        <div class="stat-label">平均复杂度</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Complexity.HighRiskCount) + `</div>
                        <div class="stat-label">极高风险函数</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%.1f%%", report.Complexity.HighRiskRatio) + `</div>
                        <div class="stat-label">高风险占比</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Complexity.TotalFunctions) + `</div>
                        <div class="stat-label">函数总数</div>
                    </div>
                </div>

                <h3 style="margin: 20px 0 10px;">复杂度分布直方图</h3>
                <div class="bar-chart">` + bars.String() + `
                </div>

                <h3 style="margin: 20px 0 10px;">Top 20 高复杂度函数</h3>
                <table>
                    <thead>
                        <tr>
                            <th style="text-align: center;">排名</th>
                            <th style="text-align: right;">复杂度</th>
                            <th>等级</th>
                            <th>函数</th>
                            <th>文件</th>
                        </tr>
                    </thead>
                    <tbody>` + funcRows.String() + `
                    </tbody>
                </table>
            </div>
        </div>`
}

func generateDuplicationHTML(report *models.Report) string {
	if report.Duplication == nil {
		return ""
	}

	rateColor := "#10b981"
	if report.Duplication.DuplicationRate >= 10 {
		rateColor = "#eab308"
	}
	if report.Duplication.DuplicationRate >= 20 {
		rateColor = "#ef4444"
	}

	var dupBlocks strings.Builder
	for i, b := range report.Duplication.TopDuplicates {
		var locs strings.Builder
		for _, loc := range b.Locations {
			locs.WriteString(`<div>` + loc.FilePath + `:` + fmt.Sprintf("%d", loc.StartLine) + `-` + fmt.Sprintf("%d", loc.EndLine) + `</div>`)
		}

		crossBadge := ""
		if b.IsCrossFile {
			crossBadge = ` <span class="badge badge-crossfile">跨文件</span>`
		}

		dupBlocks.WriteString(`
                <tr>
                    <td style="text-align: center;">` + fmt.Sprintf("%d", i+1) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", b.TokenLength) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", b.Occurrences) + crossBadge + `</td>
                    <td>` + locs.String() + `</td>
                </tr>`)
	}

	return `
        <div class="section">
            <div class="section-header" onclick="toggleSection(this)">
                <h2>🔍 重复代码检测</h2>
                <span class="section-toggle">▼</span>
            </div>
            <div class="section-content">
                <div class="stats-grid">
                    <div class="stat-item">
                        <div class="stat-value" style="color: ` + rateColor + `;">` + fmt.Sprintf("%.1f%%", report.Duplication.DuplicationRate) + `</div>
                        <div class="stat-label">重复率</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Duplication.TotalTokens) + `</div>
                        <div class="stat-label">总Token数</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Duplication.DuplicateTokens) + `</div>
                        <div class="stat-label">重复Token</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Duplication.BlockCount) + `</div>
                        <div class="stat-label">重复块数</div>
                    </div>
                </div>

                <h3 style="margin: 20px 0 10px;">Top 10 最大重复块</h3>
                <table>
                    <thead>
                        <tr>
                            <th style="text-align: center;">排名</th>
                            <th style="text-align: right;">Token长度</th>
                            <th style="text-align: right;">出现次数</th>
                            <th>位置</th>
                        </tr>
                    </thead>
                    <tbody>` + dupBlocks.String() + `
                    </tbody>
                </table>
            </div>
        </div>`
}

func generateDependencyHTML(report *models.Report) string {
	if report.Dependency == nil {
		return ""
	}

	var cyclesHTML strings.Builder
	if report.Dependency.CycleCount > 0 {
		cyclesHTML.WriteString(`
                <div class="cycle-list">
                    <h4 style="color: #dc2626; margin-bottom: 10px;">⚠️ 发现循环依赖:</h4>`)
		for i, cycle := range report.Dependency.Cycles {
			cyclesHTML.WriteString(`
                    <div class="cycle-item">
                        <strong>环 ` + fmt.Sprintf("%d", i+1) + `:</strong> ` + strings.Join(cycle, " → ") + `
                    </div>`)
		}
		cyclesHTML.WriteString(`
                </div>`)
	}

	var highCouplingHTML strings.Builder
	if len(report.Dependency.HighCoupling) > 0 {
		highCouplingHTML.WriteString(`
                <div class="high-priority">
                    <strong>高耦合文件 (扇出 > 15):</strong>
                    <ul style="margin-top: 10px; padding-left: 20px;">`)
		for _, f := range report.Dependency.HighCoupling {
			highCouplingHTML.WriteString(`<li><code>` + f + `</code></li>`)
		}
		highCouplingHTML.WriteString(`</ul></div>`)
	}

	var fanRows strings.Builder
	count := 10
	if len(report.Dependency.FanInOut) < count {
		count = len(report.Dependency.FanInOut)
	}
	for _, fo := range report.Dependency.FanInOut[:count] {
		fanOutBadge := ""
		if fo.FanOut > 15 {
			fanOutBadge = ` <span class="badge badge-danger">高耦合</span>`
		}
		fanRows.WriteString(`
                <tr>
                    <td><code>` + fo.File + `</code></td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", fo.FanIn) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", fo.FanOut) + fanOutBadge + `</td>
                </tr>`)
	}

	return `
        <div class="section">
            <div class="section-header" onclick="toggleSection(this)">
                <h2>🔗 依赖分析</h2>
                <span class="section-toggle">▼</span>
            </div>
            <div class="section-content">
                <div class="stats-grid">
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", len(report.Dependency.Nodes)) + `</div>
                        <div class="stat-label">节点数</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", len(report.Dependency.Edges)) + `</div>
                        <div class="stat-label">边数</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Dependency.CycleCount) + `</div>
                        <div class="stat-label">循环依赖</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Dependency.HighCouplingCount) + `</div>
                        <div class="stat-label">高耦合文件</div>
                    </div>
                </div>

                ` + cyclesHTML.String() + `
                ` + highCouplingHTML.String() + `

                <h3 style="margin: 20px 0 10px;">扇入/扇出 Top 10</h3>
                <table>
                    <thead>
                        <tr>
                            <th>文件</th>
                            <th style="text-align: right;">扇入</th>
                            <th style="text-align: right;">扇出</th>
                        </tr>
                    </thead>
                    <tbody>` + fanRows.String() + `
                    </tbody>
                </table>
            </div>
        </div>`
}

func generateHotspotsHTML(report *models.Report) string {
	if report.GitHotspots == nil {
		return ""
	}

	var highPriorityHTML strings.Builder
	if len(report.GitHotspots.HighPriorityFiles) > 0 {
		highPriorityHTML.WriteString(`
                <div class="high-priority">
                    <strong>⚡ 重构优先级最高 (高复杂度 + 高变更):</strong>
                    <ul style="margin-top: 10px; padding-left: 20px;">`)
		for _, f := range report.GitHotspots.HighPriorityFiles {
			highPriorityHTML.WriteString(`<li><code>` + f + `</code></li>`)
		}
		highPriorityHTML.WriteString(`</ul></div>`)
	}

	var hotRows strings.Builder
	for i, h := range report.GitHotspots.TopHotspots {
		hcBadge := ""
		if h.HighComplexity {
			hcBadge = ` <span class="badge badge-danger">高复杂度</span>`
		}
		hotRows.WriteString(`
                <tr>
                    <td style="text-align: center;">` + fmt.Sprintf("%d", i+1) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", h.Modifications) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", h.LinesChanged) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%.2f", h.Churn) + `</td>
                    <td>` + h.FilePath + hcBadge + `</td>
                </tr>`)
	}

	return `
        <div class="section">
            <div class="section-header" onclick="toggleSection(this)">
                <h2>🔥 Git变更热点</h2>
                <span class="section-toggle">▼</span>
            </div>
            <div class="section-content">
                <p style="color: #6b7280; margin-bottom: 15px;">时间范围: ` + report.GitHotspots.TimeRange + `</p>

                ` + highPriorityHTML.String() + `

                <h3 style="margin: 20px 0 10px;">Top 20 变更热点文件</h3>
                <table>
                    <thead>
                        <tr>
                            <th style="text-align: center;">排名</th>
                            <th style="text-align: right;">修改次数</th>
                            <th style="text-align: right;">修改行数</th>
                            <th style="text-align: right;">Churn值</th>
                            <th>文件</th>
                        </tr>
                    </thead>
                    <tbody>` + hotRows.String() + `
                    </tbody>
                </table>
            </div>
        </div>`
}

func generateContributorsHTML(report *models.Report) string {
	if report.Contributors == nil {
		return ""
	}

	busFactorHTML := ""
	if report.Contributors.BusFactor == 1 {
		busFactorHTML = `
                <div class="bus-factor-1">
                    ⚠️ 警告: Bus Factor 为 1，项目存在单点人员风险！
                </div>`
	}

	var contribRows strings.Builder
	count := 10
	if len(report.Contributors.Contributors) < count {
		count = len(report.Contributors.Contributors)
	}
	for i, c := range report.Contributors.Contributors[:count] {
		barWidth := c.Contribution
		if barWidth > 100 {
			barWidth = 100
		}
		contribRows.WriteString(`
                <tr>
                    <td style="text-align: center;">` + fmt.Sprintf("%d", i+1) + `</td>
                    <td>` + c.Name + `</td>
                    <td>` + c.Email + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", c.AddedLines) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", c.CommitCount) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", c.ActiveFiles) + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%.1f%%", c.Contribution) + `</td>
                    <td style="width: 150px;">
                        <div style="background: #e5e7eb; height: 8px; border-radius: 4px; overflow: hidden;">
                            <div style="background: #06b6d4; height: 100%; width: ` + fmt.Sprintf("%.1f", barWidth) + `%;"></div>
                        </div>
                    </td>
                </tr>`)
	}

	return `
        <div class="section">
            <div class="section-header" onclick="toggleSection(this)">
                <h2>👥 作者贡献分析</h2>
                <span class="section-toggle">▼</span>
            </div>
            <div class="section-content">
                <div class="stats-grid">
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Contributors.BusFactor) + `</div>
                        <div class="stat-label">Bus Factor</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", len(report.Contributors.Contributors)) + `</div>
                        <div class="stat-label">贡献者数量</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">` + fmt.Sprintf("%d", report.Contributors.TotalLines) + `</div>
                        <div class="stat-label">总新增行数</div>
                    </div>
                </div>

                ` + busFactorHTML + `

                <h3 style="margin: 20px 0 10px;">贡献排行</h3>
                <table>
                    <thead>
                        <tr>
                            <th style="text-align: center;">排名</th>
                            <th>作者</th>
                            <th>邮箱</th>
                            <th style="text-align: right;">新增行数</th>
                            <th style="text-align: right;">提交次数</th>
                            <th style="text-align: right;">活跃文件</th>
                            <th style="text-align: right;">贡献占比</th>
                            <th></th>
                        </tr>
                    </thead>
                    <tbody>` + contribRows.String() + `
                    </tbody>
                </table>
            </div>
        </div>`
}

func generateTrendHTML(report *models.Report) string {
	if report.Trend == nil || len(report.Trend.Months) == 0 {
		return ""
	}

	var trendRows strings.Builder
	for _, m := range report.Trend.Months {
		if m.CodeLines == 0 {
			continue
		}

		growthHTML := formatGrowthHTML(m.Growth)
		compHTML := formatGrowthHTML(m.ComplexityGrowth)
		dupHTML := formatGrowthHTML(m.DuplicationGrowth)

		trendRows.WriteString(`
                <tr>
                    <td>` + m.Month + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%d", m.CodeLines) + `</td>
                    <td style="text-align: right;">` + growthHTML + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%.2f", m.AvgComplexity) + `</td>
                    <td style="text-align: right;">` + compHTML + `</td>
                    <td style="text-align: right;">` + fmt.Sprintf("%.1f%%", m.DuplicationRate) + `</td>
                    <td style="text-align: right;">` + dupHTML + `</td>
                </tr>`)
	}

	return `
        <div class="section">
            <div class="section-header" onclick="toggleSection(this)">
                <h2>📊 趋势分析</h2>
                <span class="section-toggle">▼</span>
            </div>
            <div class="section-content">
                <table>
                    <thead>
                        <tr>
                            <th>月份</th>
                            <th style="text-align: right;">代码行数</th>
                            <th style="text-align: right;">代码增长</th>
                            <th style="text-align: right;">平均复杂度</th>
                            <th style="text-align: right;">复杂度变化</th>
                            <th style="text-align: right;">重复率</th>
                            <th style="text-align: right;">重复率变化</th>
                        </tr>
                    </thead>
                    <tbody>` + trendRows.String() + `
                    </tbody>
                </table>
            </div>
        </div>`
}

func formatGrowthHTML(v float64) string {
	if v == 0 {
		return "-"
	}
	str := fmt.Sprintf("%+.1f%%", v)
	if v > 10 {
		return `<span class="trend-up">` + str + ` ↑</span>`
	} else if v > 0 {
		return `<span style="color: #eab308;">` + str + `</span>`
	} else if v < -10 {
		return `<span class="trend-down">` + str + ` ↓</span>`
	}
	return str
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
