package complexity

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

type funcInfo struct {
	name      string
	startLine int
	endLine   int
	body      string
}

func Analyze(files []string, repoPath string, jobs int) *models.ComplexityReport {
	if len(files) == 0 {
		return &models.ComplexityReport{}
	}

	pool := utils.NewWorkerPool(jobs)
	defer pool.Close()

	mu := sync.Mutex{}
	var allFuncs []models.FunctionComplexity

	for _, file := range files {
		file := file
		lang := utils.GetLanguageByExt(file)
		if lang != utils.LangGo && lang != utils.LangPython &&
			lang != utils.LangJavaScript && lang != utils.LangTypeScript &&
			lang != utils.LangJava && lang != utils.LangRust &&
			lang != utils.LangC && lang != utils.LangCpp {
			continue
		}

		pool.Submit(func() {
			funcs := analyzeFile(file, lang)
			relPath, _ := strings.CutPrefix(file, repoPath+"/")
			if relPath == file {
				relPath = file
			}

			mu.Lock()
			for _, f := range funcs {
				fc := models.FunctionComplexity{
					FilePath:     relPath,
					FunctionName: f.name,
					Complexity:   f.complexity,
					Level:        utils.GetComplexityLevel(f.complexity),
				}
				allFuncs = append(allFuncs, fc)
			}
			mu.Unlock()
		})
	}

	pool.Wait()

	sort.Slice(allFuncs, func(i, j int) bool {
		return allFuncs[i].Complexity > allFuncs[j].Complexity
	})

	topN := 20
	if len(allFuncs) < topN {
		topN = len(allFuncs)
	}
	topComplex := allFuncs[:topN]

	totalComplexity := 0
	highRiskCount := 0
	for _, f := range allFuncs {
		totalComplexity += f.Complexity
		if f.Complexity > 20 {
			highRiskCount++
		}
	}

	avgComplexity := 0.0
	if len(allFuncs) > 0 {
		avgComplexity = utils.RoundFloat(float64(totalComplexity)/float64(len(allFuncs)), 2)
	}

	highRiskRatio := 0.0
	if len(allFuncs) > 0 {
		highRiskRatio = utils.RoundFloat(float64(highRiskCount)/float64(len(allFuncs))*100, 1)
	}

	distribution := buildDistribution(allFuncs)

	return &models.ComplexityReport{
		TotalFunctions: len(allFuncs),
		TopComplex:     topComplex,
		Distribution:   distribution,
		Average:        avgComplexity,
		HighRiskCount:  highRiskCount,
		HighRiskRatio:  highRiskRatio,
	}
}

func buildDistribution(funcs []models.FunctionComplexity) []models.ComplexityDistribution {
	buckets := []models.ComplexityDistribution{
		{Range: "1-5", Label: "简单", Count: 0},
		{Range: "6-10", Label: "中等", Count: 0},
		{Range: "11-20", Label: "复杂", Count: 0},
		{Range: "21+", Label: "极高风险", Count: 0},
	}

	for _, f := range funcs {
		switch {
		case f.Complexity <= 5:
			buckets[0].Count++
		case f.Complexity <= 10:
			buckets[1].Count++
		case f.Complexity <= 20:
			buckets[2].Count++
		default:
			buckets[3].Count++
		}
	}

	return buckets
}

type analyzedFunc struct {
	name       string
	startLine  int
	endLine    int
	complexity int
}

func analyzeFile(file string, lang utils.Language) []analyzedFunc {
	content, err := readFileContent(file)
	if err != nil {
		return nil
	}

	lines := strings.Split(content, "\n")
	funcs := extractFunctions(lines, lang)

	var results []analyzedFunc
	for _, f := range funcs {
		complexity := calculateComplexity(f.body)
		results = append(results, analyzedFunc{
			name:       f.name,
			startLine:  f.startLine,
			endLine:    f.endLine,
			complexity: complexity,
		})
	}

	return results
}

func extractFunctions(lines []string, lang utils.Language) []funcInfo {
	switch lang {
	case utils.LangGo:
		return extractGoFunctions(lines)
	case utils.LangPython:
		return extractPythonFunctions(lines)
	case utils.LangJavaScript, utils.LangTypeScript:
		return extractJSFunctions(lines)
	case utils.LangJava:
		return extractJavaFunctions(lines)
	case utils.LangRust:
		return extractRustFunctions(lines)
	case utils.LangC, utils.LangCpp:
		return extractCFunctions(lines)
	default:
		return nil
	}
}

func extractGoFunctions(lines []string) []funcInfo {
	var funcs []funcInfo
	funcRegex := regexp.MustCompile(`^func\s+(?:\([^)]+\)\s*)?(\w+)\s*\(`)
	methodRegex := regexp.MustCompile(`^func\s+\([^)]+\)\s*(\w+)\s*\(`)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "func ") {
			var name string
			if m := methodRegex.FindStringSubmatch(line); m != nil {
				name = "(method) " + m[1]
			} else if m := funcRegex.FindStringSubmatch(line); m != nil {
				name = m[1]
			} else {
				continue
			}

			startLine := i + 1
			body, endLine := extractBraceBlock(lines, i)
			if body != "" {
				funcs = append(funcs, funcInfo{
					name:      name,
					startLine: startLine,
					endLine:   endLine,
					body:      body,
				})
				i = endLine - 1
			}
		}
	}
	return funcs
}

func extractPythonFunctions(lines []string) []funcInfo {
	var funcs []funcInfo
	defRegex := regexp.MustCompile(`^(def\s+|async\s+def\s+)(\w+)\s*\(`)
	classRegex := regexp.MustCompile(`^class\s+\w+`)

	currentClass := ""
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if m := classRegex.FindStringSubmatch(trimmed); m != nil {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 0 {
				className := strings.TrimSpace(strings.TrimSuffix(parts[0], ":"))
				className = strings.Replace(className, "class ", "", 1)
				className = strings.TrimSpace(strings.Split(className, "(")[0])
				currentClass = className + "."
			}
			continue
		}

		if indent == 0 && trimmed != "" && !strings.HasPrefix(trimmed, "class") && !strings.HasPrefix(trimmed, "def") {
			currentClass = ""
		}

		if m := defRegex.FindStringSubmatch(trimmed); m != nil {
			name := currentClass + m[2]
			startLine := i + 1

			endLine := i
			funcIndent := indent
			for j := i + 1; j < len(lines); j++ {
				nextLine := lines[j]
				nextTrimmed := strings.TrimSpace(nextLine)
				if nextTrimmed == "" {
					continue
				}
				nextIndent := len(nextLine) - len(strings.TrimLeft(nextLine, " \t"))
				if nextIndent <= funcIndent && !strings.HasPrefix(nextTrimmed, "#") {
					endLine = j
					break
				}
				endLine = j + 1
			}

			var bodyLines []string
			for j := i; j < endLine; j++ {
				bodyLines = append(bodyLines, lines[j])
			}
			body := strings.Join(bodyLines, "\n")

			funcs = append(funcs, funcInfo{
				name:      name,
				startLine: startLine,
				endLine:   endLine,
				body:      body,
			})
			i = endLine - 1
		}
	}
	return funcs
}

func extractJSFunctions(lines []string) []funcInfo {
	var funcs []funcInfo
	funcRegex := regexp.MustCompile(`(?:^|\s)(function\s+)(\w+)\s*\(`)
	arrowRegex := regexp.MustCompile(`(?:const|let|var)?\s*(\w+)\s*[:=]\s*(?:async\s+)?\([^)]*\)\s*=>`)
	methodRegex := regexp.MustCompile(`^\s*(\w+)\s*\([^)]*\)\s*\{`)
	classMethodRegex := regexp.MustCompile(`^\s*(?:static\s+)?(\w+)\s*\([^)]*\)\s*\{`)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		var name string
		var found bool

		if m := funcRegex.FindStringSubmatch(trimmed); m != nil {
			name = m[2]
			found = true
		} else if m := arrowRegex.FindStringSubmatch(trimmed); m != nil {
			name = m[1] + " (arrow)"
			found = true
		} else if m := classMethodRegex.FindStringSubmatch(trimmed); m != nil &&
			!strings.HasPrefix(trimmed, "if") && !strings.HasPrefix(trimmed, "for") &&
			!strings.HasPrefix(trimmed, "while") && !strings.HasPrefix(trimmed, "switch") {
			name = m[1] + " (method)"
			found = true
		} else if m := methodRegex.FindStringSubmatch(trimmed); m != nil &&
			!strings.HasPrefix(trimmed, "if") && !strings.HasPrefix(trimmed, "for") &&
			!strings.HasPrefix(trimmed, "while") && !strings.HasPrefix(trimmed, "switch") &&
			!strings.HasPrefix(trimmed, "function") {
			name = m[1]
			found = true
		}

		if found && name != "" {
			startLine := i + 1
			body, endLine := extractBraceBlock(lines, i)
			if body != "" {
				funcs = append(funcs, funcInfo{
					name:      name,
					startLine: startLine,
					endLine:   endLine,
					body:      body,
				})
				i = endLine - 1
			}
		}
	}
	return funcs
}

func extractJavaFunctions(lines []string) []funcInfo {
	var funcs []funcInfo
	methodRegex := regexp.MustCompile(`(?:public|private|protected|static|final|abstract|synchronized|\s)+\s+(?:<[^>]+>\s+)?(?:[\w<>\[\]]+)\s+(\w+)\s*\([^)]*\)\s*(?:throws\s+[\w,\s]+)?\s*\{`)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if m := methodRegex.FindStringSubmatch(line); m != nil {
			name := m[1]
			startLine := i + 1
			body, endLine := extractBraceBlock(lines, i)
			if body != "" {
				funcs = append(funcs, funcInfo{
					name:      name,
					startLine: startLine,
					endLine:   endLine,
					body:      body,
				})
				i = endLine - 1
			}
		}
	}
	return funcs
}

func extractRustFunctions(lines []string) []funcInfo {
	var funcs []funcInfo
	fnRegex := regexp.MustCompile(`^fn\s+(\w+)\s*\(`)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if m := fnRegex.FindStringSubmatch(line); m != nil {
			name := m[1]
			startLine := i + 1
			body, endLine := extractBraceBlock(lines, i)
			if body != "" {
				funcs = append(funcs, funcInfo{
					name:      name,
					startLine: startLine,
					endLine:   endLine,
					body:      body,
				})
				i = endLine - 1
			}
		}
	}
	return funcs
}

func extractCFunctions(lines []string) []funcInfo {
	var funcs []funcInfo
	funcRegex := regexp.MustCompile(`^(?:[\w\s\*]+?)\s+(\w+)\s*\([^)]*\)\s*\{`)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "{") && !strings.HasPrefix(line, "if") &&
			!strings.HasPrefix(line, "for") && !strings.HasPrefix(line, "while") &&
			!strings.HasPrefix(line, "switch") && !strings.HasPrefix(line, "else") &&
			!strings.HasPrefix(line, "struct") && !strings.HasPrefix(line, "class") &&
			!strings.HasPrefix(line, "enum") && !strings.HasPrefix(line, "typedef") {

			if m := funcRegex.FindStringSubmatch(line); m != nil {
				name := m[1]
				startLine := i + 1
				body, endLine := extractBraceBlock(lines, i)
				if body != "" {
					funcs = append(funcs, funcInfo{
						name:      name,
						startLine: startLine,
						endLine:   endLine,
						body:      body,
					})
					i = endLine - 1
				}
			}
		}
	}
	return funcs
}

func extractBraceBlock(lines []string, startIdx int) (string, int) {
	depth := 0
	foundBrace := false
	var body []string

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		body = append(body, line)

		for _, ch := range line {
			if ch == '{' {
				depth++
				foundBrace = true
			} else if ch == '}' {
				depth--
			}
		}

		if foundBrace && depth == 0 {
			return strings.Join(body, "\n"), i + 1
		}
	}

	return "", len(lines)
}

func calculateComplexity(body string) int {
	complexity := 1

	body = removeStringsAndComments(body)

	keywords := []string{
		`\bif\b`, `\bfor\b`, `\bwhile\b`, `\bswitch\b`, `\bcase\b`,
		`\bcatch\b`, `\bthrow\b`, `\b&&`, `\|\|`,
		`\belse\s+if\b`, `\belse\b`,
		`\btry\b`, `\bexcept\b`, `\bfinally\b`,
		`\bdo\b`, `\?\s*[^:]`,
	}

	for _, kw := range keywords {
		re := regexp.MustCompile(kw)
		matches := re.FindAllStringIndex(body, -1)
		complexity += len(matches)
	}

	complexity -= countElseIfOverlap(body)

	return complexity
}

func countElseIfOverlap(body string) int {
	elseIfRe := regexp.MustCompile(`\belse\s+if\b`)
	matches := elseIfRe.FindAllStringIndex(body, -1)
	return len(matches)
}

func removeStringsAndComments(code string) string {
	var result []rune
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false
	runes := []rune(code)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}

		if inBlockComment {
			if ch == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inSingle || inDouble || inBacktick {
			if ch == '\\' && i+1 < len(runes) {
				i++
				continue
			}
			if inSingle && ch == '\'' {
				inSingle = false
			} else if inDouble && ch == '"' {
				inDouble = false
			} else if inBacktick && ch == '`' {
				inBacktick = false
			}
			continue
		}

		if ch == '/' && i+1 < len(runes) {
			if runes[i+1] == '/' {
				inLineComment = true
				i++
				continue
			} else if runes[i+1] == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		if ch == '\'' {
			inSingle = true
		} else if ch == '"' {
			inDouble = true
		} else if ch == '`' {
			inBacktick = true
		} else {
			result = append(result, ch)
		}
	}

	return string(result)
}

func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
