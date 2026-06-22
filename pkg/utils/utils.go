package utils

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Language string

const (
	LangGo         Language = "Go"
	LangPython     Language = "Python"
	LangJavaScript Language = "JavaScript"
	LangTypeScript Language = "TypeScript"
	LangJava       Language = "Java"
	LangRust       Language = "Rust"
	LangC          Language = "C"
	LangCpp        Language = "C++"
	LangUnknown    Language = "Unknown"
)

var extToLang = map[string]Language{
	".go":    LangGo,
	".py":    LangPython,
	".js":    LangJavaScript,
	".jsx":   LangJavaScript,
	".ts":    LangTypeScript,
	".tsx":   LangTypeScript,
	".java":  LangJava,
	".rs":    LangRust,
	".c":     LangC,
	".h":     LangC,
	".cpp":   LangCpp,
	".cc":    LangCpp,
	".cxx":   LangCpp,
	".hpp":   LangCpp,
	".hh":    LangCpp,
}

func GetLanguageByExt(path string) Language {
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := extToLang[ext]; ok {
		return lang
	}
	return LangUnknown
}

func DefaultSkipDirs() []string {
	return []string{
		".git",
		"node_modules",
		"vendor",
		"dist",
		"build",
		"target",
		".idea",
		".vscode",
		"__pycache__",
		".venv",
		"venv",
	}
}

func WalkFiles(root string, skipDirs []string) ([]string, error) {
	var files []string
	skipMap := make(map[string]bool)
	for _, d := range skipDirs {
		skipMap[d] = true
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if skipMap[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext == "" {
			return nil
		}

		if GetLanguageByExt(path) != LangUnknown {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

type WorkerPool struct {
	jobs    chan func()
	wg      sync.WaitGroup
	quit    chan struct{}
	closed  bool
	closeMu sync.Mutex
}

func NewWorkerPool(size int) *WorkerPool {
	if size <= 0 {
		size = 1
	}

	pool := &WorkerPool{
		jobs: make(chan func(), size*2),
		quit: make(chan struct{}),
	}

	for i := 0; i < size; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			if job != nil {
				job()
			}
		case <-p.quit:
			return
		}
	}
}

func (p *WorkerPool) Submit(job func()) {
	p.closeMu.Lock()
	defer p.closeMu.Unlock()
	if !p.closed {
		p.jobs <- job
	}
}

func (p *WorkerPool) Wait() {
	p.closeMu.Lock()
	alreadyClosed := p.closed
	if !p.closed {
		close(p.jobs)
		p.closed = true
	}
	p.closeMu.Unlock()
	if !alreadyClosed {
		p.wg.Wait()
	}
}

func (p *WorkerPool) Close() {
	p.closeMu.Lock()
	alreadyClosed := p.closed
	if !p.closed {
		close(p.jobs)
		p.closed = true
	}
	p.closeMu.Unlock()
	if !alreadyClosed {
		close(p.quit)
		p.wg.Wait()
	}
}

func RoundFloat(val float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Round(val*shift) / shift
}

func GetComplexityLevel(complexity int) string {
	switch {
	case complexity <= 5:
		return "简单"
	case complexity <= 10:
		return "中等"
	case complexity <= 20:
		return "复杂"
	default:
		return "极高风险"
	}
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripColor(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func FormatFilepath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func FormatPercent(p float64) string {
	return C("("+Sprintf("%.1f%%", p)+")", ColorBrightWhite)
}

func Sprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
}

func ComplexityColor(c int) string {
	switch {
	case c <= 5:
		return ColorBrightGreen
	case c <= 10:
		return ColorBrightYellow
	case c <= 20:
		return ColorBrightMagenta
	default:
		return ColorBrightRed
	}
}

func PadLeft(s string, width int) string {
	s = StripColor(s)
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

func PadRight(s string, width int) string {
	s = StripColor(s)
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func PadCenter(s string, width int) string {
	s = StripColor(s)
	if len(s) >= width {
		return s
	}
	padding := width - len(s)
	left := padding / 2
	right := padding - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
