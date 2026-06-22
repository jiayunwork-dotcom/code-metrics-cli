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
	inString := false
	stringDelim := rune(0)
	lineCommentStart, blockCommentStart, blockCommentEnd := getCommentMarkers(lang)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			blank++
			continue
		}

		lineHasComment := false
		lineIsOnlyComment := true

		runes := []rune(line)
		for i := 0; i < len(runes); i++ {
			ch := runes[i]

			if inBlockComment {
				lineHasComment = true
				if blockCommentEnd != "" && i+len(blockCommentEnd) <= len(runes) {
					if string(runes[i:i+len(blockCommentEnd)]) == blockCommentEnd {
						inBlockComment = false
						i += len(blockCommentEnd) - 1
					}
				}
				continue
			}

			if inString {
				lineIsOnlyComment = false
				if ch == '\\' && i+1 < len(runes) {
					i++
					continue
				}
				if ch == stringDelim {
					inString = false
				}
				continue
			}

			if ch == '"' || ch == '\'' || ch == '`' {
				inString = true
				stringDelim = ch
				lineIsOnlyComment = false
				continue
			}

			if blockCommentStart != "" && i+len(blockCommentStart) <= len(runes) {
				if string(runes[i:i+len(blockCommentStart)]) == blockCommentStart {
					lineHasComment = true
					remaining := strings.TrimSpace(string(runes[i:]))
					if strings.TrimSpace(string(runes[:i])) != "" {
						lineIsOnlyComment = false
					}
					if blockCommentEnd != "" && strings.Contains(remaining, blockCommentEnd) {
						afterIdx := strings.Index(remaining, blockCommentEnd) + len(blockCommentEnd)
						after := strings.TrimSpace(remaining[afterIdx:])
						if after != "" {
							lineIsOnlyComment = false
						}
					} else if blockCommentEnd != "" {
						inBlockComment = true
					}
					break
				}
			}

			if lineCommentStart != "" && i+len(lineCommentStart) <= len(runes) {
				if string(runes[i:i+len(lineCommentStart)]) == lineCommentStart {
					lineHasComment = true
					if strings.TrimSpace(string(runes[:i])) != "" {
						lineIsOnlyComment = false
					}
					break
				}
			}

			if !strings.ContainsRune(" \t\r\n", ch) {
				lineIsOnlyComment = false
			}
		}

		if lineIsOnlyComment && lineHasComment {
			comment++
		} else {
			code++
		}
	}

	return code, comment, blank
}

func getCommentMarkers(lang utils.Language) (lineStart, blockStart, blockEnd string) {
	switch lang {
	case utils.LangGo, utils.LangJavaScript, utils.LangTypeScript,
		utils.LangJava, utils.LangRust, utils.LangC, utils.LangCpp:
		return "//", "/*", "*/"
	case utils.LangPython:
		return "#", "\"\"\"", "\"\"\""
	default:
		return "", "", ""
	}
}
