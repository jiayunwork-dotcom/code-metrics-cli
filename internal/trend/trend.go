package trend

import (
	"fmt"
	"time"

	"github.com/code-metrics/cli/internal/complexity"
	"github.com/code-metrics/cli/internal/duplication"
	"github.com/code-metrics/cli/internal/metrics"
	gitpkg "github.com/code-metrics/cli/internal/git"
	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

func Analyze(repoPath string, opts *models.AnalyzerOptions) *models.TrendReport {
	if !gitpkg.IsGitRepo(repoPath) {
		return &models.TrendReport{}
	}

	originalBranch, err := gitpkg.GetCurrentBranch(repoPath)
	if err != nil {
		return &models.TrendReport{}
	}
	defer func() {
		gitpkg.CheckoutCommit(repoPath, originalBranch)
	}()

	months := opts.Months
	if months <= 0 {
		months = 6
	}

	var trendMonths []models.TrendMonth
	now := time.Now()

	for i := months - 1; i >= 0; i-- {
		year, month := getYearMonth(now, i)
		monthStr := fmt.Sprintf("%04d-%02d", year, month)

		commit, err := gitpkg.GetLastCommitOfMonth(repoPath, year, month)
		if err != nil || commit == "" {
			trendMonths = append(trendMonths, models.TrendMonth{
				Month: monthStr,
			})
			continue
		}

		err = gitpkg.CheckoutCommit(repoPath, commit)
		if err != nil {
			trendMonths = append(trendMonths, models.TrendMonth{
				Month: monthStr,
			})
			continue
		}

		files, err := utils.WalkFiles(repoPath, utils.DefaultSkipDirs())
		if err != nil || len(files) == 0 {
			trendMonths = append(trendMonths, models.TrendMonth{
				Month: monthStr,
			})
			continue
		}

		metricsReport := metrics.Analyze(files, repoPath, 1)
		complexityReport := complexity.Analyze(files, repoPath, 1)
		duplicationReport := duplication.Analyze(files, repoPath, 50, 1)

		trendMonths = append(trendMonths, models.TrendMonth{
			Month:           monthStr,
			CodeLines:       metricsReport.Total.CodeLines,
			AvgComplexity:   complexityReport.Average,
			DuplicationRate: duplicationReport.DuplicationRate,
		})
	}

	for i := 1; i < len(trendMonths); i++ {
		if trendMonths[i-1].CodeLines > 0 {
			trendMonths[i].Growth = utils.RoundFloat(
				float64(trendMonths[i].CodeLines-trendMonths[i-1].CodeLines)/
					float64(trendMonths[i-1].CodeLines)*100, 1)
		}

		if trendMonths[i-1].AvgComplexity > 0 {
			trendMonths[i].ComplexityGrowth = utils.RoundFloat(
				(trendMonths[i].AvgComplexity-trendMonths[i-1].AvgComplexity)/
					trendMonths[i-1].AvgComplexity*100, 1)
		}

		if trendMonths[i-1].DuplicationRate > 0 {
			trendMonths[i].DuplicationGrowth = utils.RoundFloat(
				(trendMonths[i].DuplicationRate-trendMonths[i-1].DuplicationRate)/
					trendMonths[i-1].DuplicationRate*100, 1)
		}
	}

	return &models.TrendReport{
		Months: trendMonths,
	}
}

func getYearMonth(now time.Time, monthsAgo int) (int, time.Month) {
	t := now.AddDate(0, -monthsAgo, 0)
	return t.Year(), t.Month()
}
