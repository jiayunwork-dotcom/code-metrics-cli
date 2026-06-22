package scoring

import (
	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

const (
	complexityWeight   = 0.30
	duplicationWeight  = 0.25
	dependencyWeight   = 0.20
	hotspotsWeight     = 0.25
)

func Calculate(report *models.Report) *models.TechDebtReport {
	complexityScore := calculateComplexityScore(report.Complexity)
	duplicationScore := calculateDuplicationScore(report.Duplication)
	dependencyScore := calculateDependencyScore(report.Dependency)
	hotspotsScore := calculateHotspotsScore(report)

	totalScore := complexityScore*complexityWeight +
		duplicationScore*duplicationWeight +
		dependencyScore*dependencyWeight +
		hotspotsScore*hotspotsWeight

	totalScore = utils.RoundFloat(totalScore, 1)

	grade, color := getGrade(totalScore)

	return &models.TechDebtReport{
		Score:      totalScore,
		Grade:      grade,
		GradeColor: color,
		Breakdown: models.TechDebtBreakdown{
			Complexity:  utils.RoundFloat(complexityScore, 1),
			Duplication: utils.RoundFloat(duplicationScore, 1),
			Dependency:  utils.RoundFloat(dependencyScore, 1),
			Hotspots:    utils.RoundFloat(hotspotsScore, 1),
		},
	}
}

func calculateComplexityScore(complexity *models.ComplexityReport) float64 {
	if complexity == nil || complexity.TotalFunctions == 0 {
		return 0
	}

	highRiskRatio := complexity.HighRiskRatio / 100.0
	avgComplexity := complexity.Average

	avgScore := min(avgComplexity/20.0*100, 100)
	highRiskScore := highRiskRatio * 100

	return (avgScore + highRiskScore) / 2
}

func calculateDuplicationScore(dup *models.DuplicationReport) float64 {
	if dup == nil || dup.TotalTokens == 0 {
		return 0
	}

	rate := dup.DuplicationRate
	return min(rate/30.0*100, 100)
}

func calculateDependencyScore(dep *models.DependencyReport) float64 {
	if dep == nil || len(dep.Nodes) == 0 {
		return 0
	}

	totalFiles := len(dep.Nodes)
	cycleScore := 0.0
	if dep.CycleCount > 0 {
		cycleScore = min(float64(dep.CycleCount)/5.0*100, 100)
	}

	highCouplingRatio := 0.0
	if totalFiles > 0 {
		highCouplingRatio = float64(dep.HighCouplingCount) / float64(totalFiles)
	}
	highCouplingScore := highCouplingRatio * 100

	return (cycleScore + highCouplingScore) / 2
}

func calculateHotspotsScore(report *models.Report) float64 {
	hotspots := report.GitHotspots
	complexity := report.Complexity

	if hotspots == nil || len(hotspots.TopHotspots) == 0 {
		return 0
	}

	totalFiles := len(hotspots.TopHotspots)
	if totalFiles == 0 {
		return 0
	}

	highPriorityRatio := float64(len(hotspots.HighPriorityFiles)) / float64(totalFiles)
	highPriorityScore := highPriorityRatio * 100

	highChurnCount := 0
	if complexity != nil && complexity.TotalFunctions > 0 {
		for _, h := range hotspots.TopHotspots {
			if h.HighComplexity {
				highChurnCount++
			}
		}
	}

	highChurnRatio := 0.0
	if len(hotspots.TopHotspots) > 0 {
		highChurnRatio = float64(highChurnCount) / float64(len(hotspots.TopHotspots))
	}
	highChurnScore := highChurnRatio * 100

	return (highPriorityScore + highChurnScore) / 2
}

func getGrade(score float64) (string, string) {
	switch {
	case score <= 20:
		return "A", utils.ColorBrightGreen
	case score <= 40:
		return "B", utils.ColorBrightCyan
	case score <= 60:
		return "C", utils.ColorBrightYellow
	case score <= 80:
		return "D", utils.ColorBrightMagenta
	default:
		return "F", utils.ColorBrightRed
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
