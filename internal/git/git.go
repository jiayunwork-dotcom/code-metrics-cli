package git

import (
	"bufio"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

type fileStats struct {
	modifications int
	linesChanged  int
	addedLines    int
}

func AnalyzeHotspots(repoPath string, complexity *models.ComplexityReport, opts *models.AnalyzerOptions) *models.GitHotspotReport {
	if !IsGitRepo(repoPath) {
		return &models.GitHotspotReport{}
	}

	since := getSinceTime(opts)
	timeRange := fmt.Sprintf("最近%d个月", opts.Months)
	if opts.TimeRange != "" {
		timeRange = opts.TimeRange
	}

	fileMap := make(map[string]*fileStats)
	err := parseGitLog(repoPath, since, fileMap)
	if err != nil {
		return &models.GitHotspotReport{TimeRange: timeRange}
	}

	highComplexFiles := make(map[string]bool)
	if complexity != nil {
		for _, f := range complexity.TopComplex {
			if f.Complexity > 10 {
				highComplexFiles[f.FilePath] = true
			}
		}
	}

	var hotspots []models.HotspotFile
	for filePath, stats := range fileMap {
		churn := float64(stats.modifications) * math.Sqrt(float64(stats.linesChanged))
		churn = utils.RoundFloat(churn, 2)

		hotspots = append(hotspots, models.HotspotFile{
			FilePath:       filePath,
			Modifications:  stats.modifications,
			LinesChanged:   stats.linesChanged,
			Churn:          churn,
			HighComplexity: highComplexFiles[filePath],
		})
	}

	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].Churn > hotspots[j].Churn
	})

	topN := 20
	if len(hotspots) < topN {
		topN = len(hotspots)
	}
	topHotspots := hotspots
	if len(hotspots) > topN {
		topHotspots = hotspots[:topN]
	}

	var highPriority []string
	for _, h := range hotspots {
		if h.HighComplexity {
			highPriority = append(highPriority, h.FilePath)
		}
	}

	return &models.GitHotspotReport{
		TopHotspots:       topHotspots,
		HighPriorityFiles: highPriority,
		TimeRange:         timeRange,
	}
}

func AnalyzeContributors(repoPath string, opts *models.AnalyzerOptions) *models.ContributorReport {
	if !IsGitRepo(repoPath) {
		return &models.ContributorReport{}
	}

	since := getSinceTime(opts)

	cmd := exec.Command("git", "log", "--numstat", "--pretty=format:%H%n%an%n%ae%n%ct",
		"--since="+since, "--follow", "-M")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return &models.ContributorReport{}
	}

	type contrib struct {
		name        string
		email       string
		addedLines  int
		commitCount int
		activeFiles map[string]bool
	}

	contribMap := make(map[string]*contrib)
	totalLines := 0

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var currentAuthor *contrib
	skipNext := 0

	for scanner.Scan() {
		if skipNext > 0 {
			skipNext--
			continue
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		if _, err := strconv.ParseInt(line, 16, 64); err == nil && len(line) >= 40 {
			if !scanner.Scan() {
				break
			}
			name := scanner.Text()

			if !scanner.Scan() {
				break
			}
			email := scanner.Text()

			if !scanner.Scan() {
				break
			}

			key := email
			if c, ok := contribMap[key]; ok {
				c.commitCount++
				currentAuthor = c
			} else {
				c := &contrib{
					name:        name,
					email:       email,
					commitCount: 1,
					activeFiles: make(map[string]bool),
				}
				contribMap[key] = c
				currentAuthor = c
			}
			continue
		}

		if currentAuthor != nil {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				added, _ := strconv.Atoi(parts[0])
				_, _ = strconv.Atoi(parts[1])
				filePath := parts[2]

				currentAuthor.addedLines += added
				currentAuthor.activeFiles[filePath] = true
				totalLines += added
			}
		}
	}

	var contributors []models.Contributor
	for _, c := range contribMap {
		contribution := 0.0
		if totalLines > 0 {
			contribution = utils.RoundFloat(float64(c.addedLines)/float64(totalLines)*100, 1)
		}

		contributors = append(contributors, models.Contributor{
			Name:         c.name,
			Email:        c.email,
			AddedLines:   c.addedLines,
			CommitCount:  c.commitCount,
			ActiveFiles:  len(c.activeFiles),
			Contribution: contribution,
		})
	}

	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i].AddedLines > contributors[j].AddedLines
	})

	busFactor := calculateBusFactor(contributors, totalLines)

	return &models.ContributorReport{
		Contributors: contributors,
		BusFactor:    busFactor,
		TotalLines:   totalLines,
	}
}

func calculateBusFactor(contributors []models.Contributor, totalLines int) int {
	if totalLines == 0 {
		return 0
	}

	half := float64(totalLines) * 0.5
	cumulative := 0
	for i, c := range contributors {
		cumulative += c.AddedLines
		if float64(cumulative) >= half {
			return i + 1
		}
	}
	return len(contributors)
}

func IsGitRepo(repoPath string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = repoPath
	err := cmd.Run()
	return err == nil
}

func getSinceTime(opts *models.AnalyzerOptions) string {
	if opts.TimeRange != "" {
		return opts.TimeRange
	}
	months := opts.Months
	if months <= 0 {
		months = 6
	}
	return fmt.Sprintf("%d months ago", months)
}

func parseGitLog(repoPath string, since string, fileMap map[string]*fileStats) error {
	cmd := exec.Command("git", "log", "--numstat", "--pretty=format:COMMIT %H",
		"--since="+since, "--follow", "-M", "-C")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	renameMap := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "COMMIT") {
			continue
		}

		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		added, err := strconv.Atoi(parts[0])
		if err != nil {
			added = 0
		}
		deleted, err := strconv.Atoi(parts[1])
		if err != nil {
			deleted = 0
		}
		filePath := parts[2]

		filePath = resolveRename(filePath, renameMap)

		if strings.Contains(line, "=>") {
			oldPath, newPath := parseRename(parts)
			if oldPath != "" && newPath != "" {
				renameMap[oldPath] = newPath
				filePath = newPath
			}
		}

		if _, ok := fileMap[filePath]; !ok {
			fileMap[filePath] = &fileStats{}
		}
		fileMap[filePath].modifications++
		fileMap[filePath].linesChanged += added + deleted
		fileMap[filePath].addedLines += added
	}

	return nil
}

func resolveRename(path string, renameMap map[string]string) string {
	resolved := path
	for {
		if newPath, ok := renameMap[resolved]; ok {
			resolved = newPath
		} else {
			break
		}
	}
	return resolved
}

func parseRename(parts []string) (string, string) {
	for _, p := range parts {
		if strings.Contains(p, "=>") {
			idx := strings.Index(p, "{")
			if idx >= 0 {
				endIdx := strings.Index(p, "}")
				if endIdx >= 0 {
					inner := p[idx+1 : endIdx]
					renameParts := strings.SplitN(inner, "=>", 2)
					if len(renameParts) == 2 {
						prefix := p[:idx]
						suffix := p[endIdx+1:]
						return prefix + strings.TrimSpace(renameParts[0]) + suffix,
							prefix + strings.TrimSpace(renameParts[1]) + suffix
					}
				}
			} else {
				renameParts := strings.SplitN(p, "=>", 2)
				if len(renameParts) == 2 {
					return strings.TrimSpace(renameParts[0]), strings.TrimSpace(renameParts[1])
				}
			}
		}
	}
	return "", ""
}

func GetMonthlyCommits(repoPath string, months int) ([]time.Time, error) {
	since := fmt.Sprintf("%d months ago", months)
	cmd := exec.Command("git", "log", "--pretty=format:%ct", "--since="+since)
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var times []time.Time
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		ts, err := strconv.ParseInt(scanner.Text(), 10, 64)
		if err == nil {
			times = append(times, time.Unix(ts, 0))
		}
	}

	return times, nil
}

func GetLastCommitOfMonth(repoPath string, year int, month time.Month) (string, error) {
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1).Add(23 * time.Hour).Add(59 * time.Minute).Add(59 * time.Second)

	cmd := exec.Command("git", "log", "--pretty=format:%H",
		"--before="+lastDay.Format(time.RFC3339),
		"--after="+firstDay.AddDate(0, 0, -1).Format(time.RFC3339),
		"-1")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func CheckoutCommit(repoPath, commitHash string) error {
	cmd := exec.Command("git", "checkout", commitHash)
	cmd.Dir = repoPath
	return cmd.Run()
}

func GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
