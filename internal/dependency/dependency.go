package dependency

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

type graph struct {
	nodes []string
	edges map[string][]string
	index map[string]int
}

func newGraph() *graph {
	return &graph{
		nodes: []string{},
		edges: make(map[string][]string),
		index: make(map[string]int),
	}
}

func (g *graph) addNode(n string) {
	if _, ok := g.index[n]; !ok {
		g.index[n] = len(g.nodes)
		g.nodes = append(g.nodes, n)
	}
}

func (g *graph) addEdge(from, to string) {
	g.addNode(from)
	g.addNode(to)
	g.edges[from] = append(g.edges[from], to)
}

func Analyze(files []string, repoPath string, highFanOut int, jobs int) *models.DependencyReport {
	if len(files) == 0 {
		return &models.DependencyReport{}
	}

	g := newGraph()
	pool := utils.NewWorkerPool(jobs)
	defer pool.Close()

	mu := sync.Mutex{}
	relPathMap := make(map[string]string)

	for _, file := range files {
		file := file
		pool.Submit(func() {
			relPath, _ := strings.CutPrefix(file, repoPath+"/")
			if relPath == file {
				relPath = file
			}

			lang := utils.GetLanguageByExt(file)
			imports := parseImports(file, lang, repoPath, relPath)

			mu.Lock()
			relPathMap[file] = relPath
			g.addNode(relPath)
			for _, imp := range imports {
				g.addEdge(relPath, imp)
			}
			mu.Unlock()
		})
	}

	pool.Wait()

	var edges []models.DependencyEdge
	uniqueEdges := make(map[string]bool)

	for from, tos := range g.edges {
		sort.Strings(tos)
		for _, to := range tos {
			key := from + "->" + to
			if !uniqueEdges[key] {
				uniqueEdges[key] = true
				edges = append(edges, models.DependencyEdge{From: from, To: to})
			}
		}
	}

	cycles := findCycles(g)

	fanInOut := calculateFanInOut(g)

	var highCoupling []string
	for _, fo := range fanInOut {
		if fo.FanOut > highFanOut {
			highCoupling = append(highCoupling, fo.File)
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		return edges[i].From < edges[j].From || (edges[i].From == edges[j].From && edges[i].To < edges[j].To)
	})

	sort.Slice(fanInOut, func(i, j int) bool {
		return fanInOut[i].FanOut > fanInOut[j].FanOut
	})

	sort.Slice(highCoupling, func(i, j int) bool {
		return highCoupling[i] < highCoupling[j]
	})

	sort.Slice(g.nodes, func(i, j int) bool {
		return g.nodes[i] < g.nodes[j]
	})

	return &models.DependencyReport{
		Nodes:            g.nodes,
		Edges:            edges,
		Cycles:           cycles,
		FanInOut:         fanInOut,
		HighCoupling:     highCoupling,
		CycleCount:       len(cycles),
		HighCouplingCount: len(highCoupling),
	}
}

func parseImports(file string, lang utils.Language, repoPath, relPath string) []string {
	content, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer content.Close()

	var imports []string
	scanner := bufio.NewScanner(content)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	importRegexes := getImportRegex(lang)

	for scanner.Scan() {
		line := scanner.Text()
		imp := matchImport(line, importRegexes, lang, file, repoPath, relPath)
		if imp != "" {
			imports = append(imports, imp)
		}
	}

	return imports
}

func getImportRegex(lang utils.Language) []*regexp.Regexp {
	var regexes []*regexp.Regexp

	switch lang {
	case utils.LangGo:
		regexes = append(regexes, regexp.MustCompile(`^\s*import\s+"([^"]+)"`))
		regexes = append(regexes, regexp.MustCompile(`^\s*"([^"]+)"`))
	case utils.LangPython:
		regexes = append(regexes, regexp.MustCompile(`^\s*import\s+([\w\.]+)`))
		regexes = append(regexes, regexp.MustCompile(`^\s*from\s+([\w\.]+)\s+import`))
	case utils.LangJavaScript, utils.LangTypeScript:
		regexes = append(regexes, regexp.MustCompile(`^\s*import\s+(?:.+\s+from\s+)?['"]([^'"]+)['"]`))
		regexes = append(regexes, regexp.MustCompile(`^\s*const\s+\w+\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`))
		regexes = append(regexes, regexp.MustCompile(`^\s*var\s+\w+\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`))
		regexes = append(regexes, regexp.MustCompile(`^\s*import\s*\(\s*['"]([^'"]+)['"]`))
	case utils.LangJava:
		regexes = append(regexes, regexp.MustCompile(`^\s*import\s+([\w\.]+);`))
	case utils.LangRust:
		regexes = append(regexes, regexp.MustCompile(`^\s*use\s+([\w:]+);`))
	case utils.LangC, utils.LangCpp:
		regexes = append(regexes, regexp.MustCompile(`^\s*#\s*include\s+[<"]([^>"]+)[>"]`))
	}

	return regexes
}

func matchImport(line string, regexes []*regexp.Regexp, lang utils.Language, file, repoPath, relPath string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
		return ""
	}

	if strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
		return ""
	}

	for _, re := range regexes {
		if m := re.FindStringSubmatch(line); m != nil && len(m) >= 2 {
			return resolveImportPath(m[1], lang, file, repoPath, relPath)
		}
	}

	return ""
}

func resolveImportPath(imp string, lang utils.Language, file, repoPath, relPath string) string {
	if imp == "" {
		return ""
	}

	switch lang {
	case utils.LangGo:
		return imp
	case utils.LangPython:
		return resolvePythonImport(imp, file, repoPath, relPath)
	case utils.LangJavaScript, utils.LangTypeScript:
		return resolveJSImport(imp, file, repoPath, relPath)
	case utils.LangJava:
		return imp
	case utils.LangRust:
		return imp
	case utils.LangC, utils.LangCpp:
		return resolveCInclude(imp, file, repoPath, relPath)
	default:
		return imp
	}
}

func resolvePythonImport(imp string, file, repoPath, relPath string) string {
	parts := strings.Split(imp, ".")
	if strings.HasPrefix(imp, ".") {
		level := 0
		for _, p := range parts {
			if p == "" {
				level++
			}
			break
		}
		dir := filepath.Dir(relPath)
		for i := 1; i < level; i++ {
			dir = filepath.Dir(dir)
		}
		moduleName := strings.TrimLeft(imp, ".")
		if dir == "." {
			return moduleName
		}
		return filepath.Join(dir, moduleName)
	}
	return imp
}

func resolveJSImport(imp string, file, repoPath, relPath string) string {
	if strings.HasPrefix(imp, ".") {
		dir := filepath.Dir(relPath)
		full := filepath.Clean(filepath.Join(dir, imp))

		exts := []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}
		for _, ext := range exts {
			if strings.HasSuffix(full, ext) {
				return full
			}
			testPath := full + ext
			if fileExists(filepath.Join(repoPath, testPath)) {
				return testPath
			}
		}

		for _, ext := range exts {
			testPath := filepath.Join(full, "index"+ext)
			if fileExists(filepath.Join(repoPath, testPath)) {
				return testPath
			}
		}
		return full
	}
	return imp
}

func resolveCInclude(imp string, file, repoPath, relPath string) string {
	if !strings.HasPrefix(imp, "/") {
		dir := filepath.Dir(relPath)
		full := filepath.Clean(filepath.Join(dir, imp))
		if fileExists(filepath.Join(repoPath, full)) {
			return full
		}
	}
	return imp
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findCycles(g *graph) [][]string {
	var result [][]string
	index := 0
	indices := make(map[string]int)
	lowlink := make(map[string]int)
	onStack := make(map[string]bool)
	stack := []string{}

	var strongConnect func(v string)
	strongConnect = func(v string) {
		indices[v] = index
		lowlink[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for _, w := range g.edges[v] {
			if _, ok := indices[w]; !ok {
				strongConnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if indices[w] < lowlink[v] {
					lowlink[v] = indices[w]
				}
			}
		}

		if lowlink[v] == indices[v] {
			var component []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				component = append(component, w)
				if w == v {
					break
				}
			}
			if len(component) > 1 {
				sort.Strings(component)
				result = append(result, component)
			}
		}
	}

	for _, v := range g.nodes {
		if _, ok := indices[v]; !ok {
			strongConnect(v)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return len(result[i]) > len(result[j])
	})

	return result
}

func calculateFanInOut(g *graph) []models.FanInOut {
	fanIn := make(map[string]int)
	fanOut := make(map[string]int)

	for _, node := range g.nodes {
		fanIn[node] = 0
		fanOut[node] = 0
	}

	for from, tos := range g.edges {
		fanOut[from] += len(tos)
		for _, to := range tos {
			fanIn[to]++
		}
	}

	var result []models.FanInOut
	for _, node := range g.nodes {
		result = append(result, models.FanInOut{
			File:   node,
			FanIn:  fanIn[node],
			FanOut: fanOut[node],
		})
	}

	return result
}

func ExportDOT(report *models.DependencyReport, outputPath string) error {
	var dot strings.Builder
	dot.WriteString("digraph dependencies {\n")
	dot.WriteString("  rankdir=LR;\n")
	dot.WriteString("  node [shape=box, style=filled, fillcolor=white];\n")

	for _, node := range report.Nodes {
		label := node
		label = strings.ReplaceAll(label, "\\", "\\\\")
		label = strings.ReplaceAll(label, "\"", "\\\"")
		dot.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\"];\n", node, label))
	}

	for _, edge := range report.Edges {
		from := strings.ReplaceAll(edge.From, "\"", "\\\"")
		to := strings.ReplaceAll(edge.To, "\"", "\\\"")
		dot.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", from, to))
	}

	for i, cycle := range report.Cycles {
		dot.WriteString(fmt.Sprintf("  subgraph cluster_%d {\n", i))
		dot.WriteString("    style=filled;\n")
		dot.WriteString("    fillcolor=lightcoral;\n")
		for _, node := range cycle {
			node = strings.ReplaceAll(node, "\"", "\\\"")
			dot.WriteString(fmt.Sprintf("    \"%s\";\n", node))
		}
		dot.WriteString("  }\n")
	}

	dot.WriteString("}\n")

	return os.WriteFile(outputPath, []byte(dot.String()), 0644)
}
