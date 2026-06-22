package incremental

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/code-metrics/cli/internal/git"
	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

type IncGraph struct {
	nodes []string
	edges map[string][]string
	index map[string]int
}

func newIncGraph() *IncGraph {
	return &IncGraph{
		nodes: []string{},
		edges: make(map[string][]string),
		index: make(map[string]int),
	}
}

func (g *IncGraph) addNode(n string) {
	if _, ok := g.index[n]; !ok {
		g.index[n] = len(g.nodes)
		g.nodes = append(g.nodes, n)
	}
}

func (g *IncGraph) addEdge(from, to string) {
	g.addNode(from)
	g.addNode(to)
	g.edges[from] = append(g.edges[from], to)
}

func AnalyzeDependencyDiff(changedFiles []models.ChangedFile, opts *models.AnalyzerOptions) *models.DependencyDiffReport {
	if len(changedFiles) == 0 {
		return &models.DependencyDiffReport{}
	}

	oldGraph, newGraph := buildDependencyGraphs(changedFiles, opts)

	oldEdges := make(map[string]bool)
	for from, tos := range oldGraph.edges {
		sort.Strings(tos)
		for _, to := range tos {
			oldEdges[from+"->"+to] = true
		}
	}

	newEdges := make(map[string]bool)
	edgeMap := make(map[string]models.DependencyEdge)
	for from, tos := range newGraph.edges {
		sort.Strings(tos)
		for _, to := range tos {
			key := from + "->" + to
			newEdges[key] = true
			edgeMap[key] = models.DependencyEdge{From: from, To: to}
		}
	}

	var addedEdges []models.DependencyEdge
	var removedEdges []models.DependencyEdge

	for key := range newEdges {
		if !oldEdges[key] {
			addedEdges = append(addedEdges, edgeMap[key])
		}
	}

	for key := range oldEdges {
		if !newEdges[key] {
			parts := strings.SplitN(key, "->", 2)
			if len(parts) == 2 {
				removedEdges = append(removedEdges, models.DependencyEdge{From: parts[0], To: parts[1]})
			}
		}
	}

	oldCycles := findIncCycles(oldGraph)
	newCycles := findIncCycles(newGraph)

	oldCycleKeys := make(map[string]bool)
	for _, cycle := range oldCycles {
		key := strings.Join(cycle, ",")
		oldCycleKeys[key] = true
	}

	var newCyclesList [][]string
	for _, cycle := range newCycles {
		key := strings.Join(cycle, ",")
		if !oldCycleKeys[key] {
			newCyclesList = append(newCyclesList, cycle)
		}
	}

	sort.Slice(addedEdges, func(i, j int) bool {
		return addedEdges[i].From < addedEdges[j].From || (addedEdges[i].From == addedEdges[j].From && addedEdges[i].To < addedEdges[j].To)
	})
	sort.Slice(removedEdges, func(i, j int) bool {
		return removedEdges[i].From < removedEdges[j].From || (removedEdges[i].From == removedEdges[j].From && removedEdges[i].To < removedEdges[j].To)
	})

	return &models.DependencyDiffReport{
		AddedEdges:    addedEdges,
		RemovedEdges:  removedEdges,
		NewCycles:     newCyclesList,
		NewCycleCount: len(newCyclesList),
	}
}

func buildDependencyGraphs(changedFiles []models.ChangedFile, opts *models.AnalyzerOptions) (*IncGraph, *IncGraph) {
	oldGraph := newIncGraph()
	newGraph := newIncGraph()

	pool := utils.NewWorkerPool(opts.Jobs)
	defer pool.Close()

	mu := sync.Mutex{}
	packageToFile := make(map[string]string)

	for _, cf := range changedFiles {
		cf := cf
		if !strings.HasSuffix(cf.FilePath, ".go") {
			continue
		}
		relPath := cf.FilePath
		dir := filepath.Dir(relPath)
		packageToFile[dir] = relPath
	}

	for _, cf := range changedFiles {
		cf := cf
		pool.Submit(func() {
			lang := utils.GetLanguageByExt(cf.FilePath)
			relPath := cf.FilePath
			oldPath := cf.FilePath
			if cf.OldPath != "" {
				oldPath = cf.OldPath
			}

			if cf.ChangeType != "added" {
				oldContent, err := git.GetFileContent(opts.RepoPath, opts.DiffCommit1, oldPath)
				if err == nil && oldContent != "" {
					oldImports := parseIncImports(oldContent, lang, opts.RepoPath, relPath, packageToFile)
					mu.Lock()
					oldGraph.addNode(relPath)
					for _, imp := range oldImports {
						oldGraph.addEdge(relPath, imp)
					}
					mu.Unlock()
				}
			}

			newContent, err := git.GetFileContent(opts.RepoPath, opts.DiffCommit2, cf.FilePath)
			if err == nil && newContent != "" {
				newImports := parseIncImports(newContent, lang, opts.RepoPath, relPath, packageToFile)
				mu.Lock()
				newGraph.addNode(relPath)
				for _, imp := range newImports {
					newGraph.addEdge(relPath, imp)
				}
				mu.Unlock()
			}
		})
	}

	pool.Wait()

	return oldGraph, newGraph
}

func parseIncImports(content string, lang utils.Language, repoPath, relPath string, packageToFile map[string]string) []string {
	var imports []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	inImportBlock := false
	inMultiLineComment := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		if inMultiLineComment {
			if strings.Contains(line, "*/") {
				inMultiLineComment = false
			}
			continue
		}

		if strings.HasPrefix(trimmed, "/*") {
			if !strings.Contains(trimmed, "*/") {
				inMultiLineComment = true
			}
			continue
		}

		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		if lang == utils.LangGo {
			if strings.HasPrefix(trimmed, "import (") {
				inImportBlock = true
				continue
			}
			if inImportBlock {
				if strings.HasPrefix(trimmed, ")") {
					inImportBlock = false
					continue
				}
				re := regexp.MustCompile(`^\s*"([^"]+)"`)
				if m := re.FindStringSubmatch(line); m != nil && len(m) >= 2 {
					imp := resolveIncImportPath(m[1], lang, repoPath, relPath, packageToFile)
					if isIncValidImport(imp) {
						imports = append(imports, imp)
					}
					continue
				}
			}
			re := regexp.MustCompile(`^\s*import\s+"([^"]+)"`)
			if m := re.FindStringSubmatch(line); m != nil && len(m) >= 2 {
				imp := resolveIncImportPath(m[1], lang, repoPath, relPath, packageToFile)
				if isIncValidImport(imp) {
					imports = append(imports, imp)
				}
			}
			continue
		}

		imp := matchIncImport(line, lang, repoPath, relPath, packageToFile)
		if imp != "" && isIncValidImport(imp) {
			imports = append(imports, imp)
		}
	}

	return imports
}

func isIncValidImport(imp string) bool {
	if imp == "" {
		return false
	}

	invalidPatterns := []string{
		".go", ".py", ".js", ".ts", ".java", ".rs", ".c", ".cpp", ".h", ".hpp",
		"node_modules", "vendor", "dist", "build", "target",
	}

	for _, p := range invalidPatterns {
		if imp == p {
			return false
		}
	}

	if len(imp) < 3 {
		return false
	}

	if regexp.MustCompile(`^[+\-*/%=<>!&|^~]+$`).MatchString(imp) {
		return false
	}

	if regexp.MustCompile(`^\d+$`).MatchString(imp) {
		return false
	}

	return true
}

func matchIncImport(line string, lang utils.Language, repoPath, relPath string, packageToFile map[string]string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}

	var re *regexp.Regexp

	switch lang {
	case utils.LangPython:
		if strings.HasPrefix(trimmed, "import ") {
			re = regexp.MustCompile(`^\s*import\s+([\w\.]+)`)
		} else if strings.HasPrefix(trimmed, "from ") {
			re = regexp.MustCompile(`^\s*from\s+([\w\.]+)\s+import`)
		}
	case utils.LangJavaScript, utils.LangTypeScript:
		if strings.HasPrefix(trimmed, "import ") {
			re = regexp.MustCompile(`^\s*import\s+(?:.+\s+from\s+)?['"]([^'"]+)['"]`)
		} else if strings.Contains(trimmed, "require(") {
			re = regexp.MustCompile(`^\s*(?:const|var|let)\s+\w+\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
		}
	case utils.LangJava:
		if strings.HasPrefix(trimmed, "import ") {
			re = regexp.MustCompile(`^\s*import\s+([\w\.]+);`)
		}
	case utils.LangRust:
		if strings.HasPrefix(trimmed, "use ") {
			re = regexp.MustCompile(`^\s*use\s+([\w:]+);`)
		}
	case utils.LangC, utils.LangCpp:
		if strings.HasPrefix(trimmed, "#include") {
			re = regexp.MustCompile(`^\s*#\s*include\s+[<"]([^>"]+)[>"]`)
		}
	}

	if re != nil {
		if m := re.FindStringSubmatch(line); m != nil && len(m) >= 2 {
			return resolveIncImportPath(m[1], lang, repoPath, relPath, packageToFile)
		}
	}

	return ""
}

func resolveIncImportPath(imp string, lang utils.Language, repoPath, relPath string, packageToFile map[string]string) string {
	if imp == "" {
		return ""
	}

	switch lang {
	case utils.LangGo:
		return resolveIncGoImportPath(imp, packageToFile)
	case utils.LangPython:
		return resolveIncPythonImport(imp, relPath)
	case utils.LangJavaScript, utils.LangTypeScript:
		return resolveIncJSImport(imp, repoPath, relPath)
	case utils.LangJava:
		return imp
	case utils.LangRust:
		return imp
	case utils.LangC, utils.LangCpp:
		return resolveIncCInclude(imp, repoPath, relPath)
	default:
		return imp
	}
}

func resolveIncGoImportPath(imp string, packageToFile map[string]string) string {
	if imp == "" {
		return imp
	}

	for pkgDir, filePath := range packageToFile {
		if strings.HasSuffix(imp, pkgDir) {
			return filepath.Dir(filePath) + "/" + filepath.Base(filePath)
		}
	}

	parts := strings.Split(imp, "/")
	for i := len(parts); i >= 1; i-- {
		suffix := strings.Join(parts[len(parts)-i:], "/")
		if filePath, ok := packageToFile[suffix]; ok {
			return filepath.Dir(filePath) + "/" + filepath.Base(filePath)
		}
	}

	return imp
}

func resolveIncPythonImport(imp string, relPath string) string {
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

func resolveIncJSImport(imp string, repoPath, relPath string) string {
	if strings.HasPrefix(imp, ".") {
		dir := filepath.Dir(relPath)
		full := filepath.Clean(filepath.Join(dir, imp))

		exts := []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}
		for _, ext := range exts {
			if strings.HasSuffix(full, ext) {
				return full
			}
			testPath := full + ext
			if fileIncExists(filepath.Join(repoPath, testPath)) {
				return testPath
			}
		}

		for _, ext := range exts {
			testPath := filepath.Join(full, "index"+ext)
			if fileIncExists(filepath.Join(repoPath, testPath)) {
				return testPath
			}
		}
		return full
	}
	return imp
}

func resolveIncCInclude(imp string, repoPath, relPath string) string {
	if !strings.HasPrefix(imp, "/") {
		dir := filepath.Dir(relPath)
		full := filepath.Clean(filepath.Join(dir, imp))
		if fileIncExists(filepath.Join(repoPath, full)) {
			return full
		}
	}
	return imp
}

func fileIncExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findIncCycles(g *IncGraph) [][]string {
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
