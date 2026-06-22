package metrics

import (
	"bufio"
	"os"
	"strings"
	"sync"

	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

type lineCounter struct {
	code    int
	comment int
	blank   int
}

func Analyze(files []string, repoPath string, jobs int) *models.MetricsReport {
	if len(files) == 0 {
		return &models.MetricsReport{
			ByLanguage: []models.LanguageMetrics{},
			Total:      models.Totals{},
		}
	}

	pool := utils.NewWorkerPool(jobs)
	defer pool.Close()

	mu := sync.Mutex{}
	langMap := make(map[string]*lineCounter)

	for _, file := range files {
		file := file
		pool.Submit(func() {
			lang := string(utils.GetLanguageByExt(file))
			code, comment, blank := countLines(file)

			mu.Lock()
			if _, ok := langMap[lang]; !ok {
				langMap[lang] = &lineCounter{}
			}
			langMap[lang].code += code
			langMap[lang].comment += comment
			langMap[lang].blank += blank
			mu.Unlock()
		})
	}

	pool.Wait()

	langOrder := []string{"Go", "Python", "JavaScript", "TypeScript", "Java", "Rust", "C", "C++"}
	var byLang []models.LanguageMetrics
	var total models.Totals
	fileCountMap := countFilesByLang(files)

	for _, lang := range langOrder {
		if lc, ok := langMap[lang]; ok {
			fileCount := fileCountMap[lang]
			totalLines := lc.code + lc.comment + lc.blank
			commentRatio := 0.0
			if totalLines > 0 {
				commentRatio = utils.RoundFloat(float64(lc.comment)/float64(totalLines)*100, 1)
			}

			byLang = append(byLang, models.LanguageMetrics{
				Language:     lang,
				FileCount:    fileCount,
				CodeLines:    lc.code,
				CommentLines: lc.comment,
				BlankLines:   lc.blank,
				CommentRatio: commentRatio,
			})

			total.FileCount += fileCount
			total.CodeLines += lc.code
			total.CommentLines += lc.comment
			total.BlankLines += lc.blank
		}
	}

	totalLines := total.CodeLines + total.CommentLines + total.BlankLines
	if totalLines > 0 {
		total.CommentRatio = utils.RoundFloat(float64(total.CommentLines)/float64(totalLines)*100, 1)
	}

	return &models.MetricsReport{
		ByLanguage: byLang,
		Total:      total,
	}
}

func countFilesByLang(files []string) map[string]int {
	counts := make(map[string]int)
	for _, f := range files {
		lang := string(utils.GetLanguageByExt(f))
		counts[lang]++
	}
	return counts
}

func countLines(path string) (code, comment, blank int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, 0
	}
	defer file.Close()

	lang := utils.GetLanguageByExt(path)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	inBlockComment := false
	commentStart, commentEnd := getCommentMarkers(lang)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			blank++
			continue
		}

		isCommentLine := false
		hasCode := false

		if commentEnd != "" {
			if inBlockComment {
				isCommentLine = true
				if strings.Contains(line, commentEnd) {
					inBlockComment = false
					after := line[strings.Index(line, commentEnd)+len(commentEnd):]
					if strings.TrimSpace(after) != "" {
						hasCode = true
					}
				}
			} else if strings.HasPrefix(trimmed, commentStart) && commentStart != "" {
				isCommentLine = true
			} else if strings.Contains(line, commentStart) {
				idx := strings.Index(line, commentStart)
				if !strings.Contains(line[:idx], "string_literal_placeholder") {
					before := strings.TrimSpace(line[:idx])
					if before == "" {
						isCommentLine = true
					} else {
						hasCode = true
					}
					if strings.Contains(line[idx:], commentEnd) {
					} else {
						inBlockComment = true
					}
				}
			}
		} else if commentStart != "" {
			if strings.HasPrefix(trimmed, commentStart) {
				isCommentLine = true
			} else if strings.Contains(line, commentStart) {
				hasCode = true
			}
		}

		if isCommentLine && !hasCode {
			comment++
		} else {
			code++
		}
	}

	return code, comment, blank
}

func getCommentMarkers(lang utils.Language) (lineStart, blockEnd string) {
	switch lang {
	case utils.LangGo, utils.LangJavaScript, utils.LangTypeScript,
		utils.LangJava, utils.LangRust, utils.LangC, utils.LangCpp:
		return "//", "*/"
	case utils.LangPython:
		return "#", ""
	default:
		return "", ""
	}
}
