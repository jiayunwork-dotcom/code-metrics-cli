package models

import "time"

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

type AnalyzerOptions struct {
	RepoPath      string
	Format        string
	OutputFile    string
	CIMode        bool
	TrendAnalysis bool
	TimeRange     string
	Months        int
	Jobs          int
	ConfigFile    string
	MinTokenLen   int
	HighFanOut    int
}

type QualityGates struct {
	MaxAvgComplexity   float64 `yaml:"max_avg_complexity"`
	MaxDuplicationRate float64 `yaml:"max_duplication_rate"`
	AllowCycles        bool    `yaml:"allow_cycles"`
}

type Config struct {
	QualityGates QualityGates `yaml:"quality_gates"`
}

type QualityGateResult struct {
	Passed     bool     `json:"passed"`
	Violations []string `json:"violations,omitempty"`
}

type Report struct {
	RepoPath      string             `json:"repo_path"`
	GeneratedAt   time.Time          `json:"generated_at"`
	Metrics       *MetricsReport     `json:"metrics,omitempty"`
	Complexity    *ComplexityReport  `json:"complexity,omitempty"`
	Duplication   *DuplicationReport `json:"duplication,omitempty"`
	Dependency    *DependencyReport  `json:"dependency,omitempty"`
	GitHotspots   *GitHotspotReport  `json:"git_hotspots,omitempty"`
	Contributors  *ContributorReport `json:"contributors,omitempty"`
	TechDebt      *TechDebtReport    `json:"tech_debt,omitempty"`
	Trend         *TrendReport       `json:"trend,omitempty"`
	QualityGates  *QualityGateResult `json:"quality_gates,omitempty"`
}

type LanguageMetrics struct {
	Language     string  `json:"language"`
	FileCount    int     `json:"file_count"`
	CodeLines    int     `json:"code_lines"`
	CommentLines int     `json:"comment_lines"`
	BlankLines   int     `json:"blank_lines"`
	CommentRatio float64 `json:"comment_ratio"`
}

type Totals struct {
	FileCount    int     `json:"file_count"`
	CodeLines    int     `json:"code_lines"`
	CommentLines int     `json:"comment_lines"`
	BlankLines   int     `json:"blank_lines"`
	CommentRatio float64 `json:"comment_ratio"`
}

type MetricsReport struct {
	ByLanguage []LanguageMetrics `json:"by_language"`
	Total      Totals            `json:"total"`
}

type FunctionComplexity struct {
	FilePath     string `json:"file_path"`
	FunctionName string `json:"function_name"`
	Complexity   int    `json:"complexity"`
	Level        string `json:"level"`
}

type ComplexityDistribution struct {
	Range string `json:"range"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type ComplexityReport struct {
	Average       float64                  `json:"average"`
	HighRiskCount int                      `json:"high_risk_count"`
	HighRiskRatio float64                  `json:"high_risk_ratio"`
	TotalFunctions int                     `json:"total_functions"`
	TopComplex    []FunctionComplexity     `json:"top_complex"`
	Distribution  []ComplexityDistribution `json:"distribution"`
}

type DuplicateLocation struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type DuplicateBlock struct {
	TokenLength int                `json:"token_length"`
	Occurrences int                `json:"occurrences"`
	IsCrossFile bool               `json:"is_cross_file"`
	Locations   []DuplicateLocation `json:"locations"`
}

type DuplicationReport struct {
	DuplicationRate  float64         `json:"duplication_rate"`
	TotalTokens      int             `json:"total_tokens"`
	DuplicateTokens  int             `json:"duplicate_tokens"`
	BlockCount       int             `json:"block_count"`
	SameFileBlocks   int             `json:"same_file_blocks"`
	CrossFileBlocks  int             `json:"cross_file_blocks"`
	TopDuplicates    []DuplicateBlock `json:"top_duplicates"`
}

type DependencyEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type FanInOut struct {
	File   string `json:"file"`
	FanIn  int    `json:"fan_in"`
	FanOut int    `json:"fan_out"`
}

type DependencyReport struct {
	Nodes             []string          `json:"nodes"`
	Edges             []DependencyEdge  `json:"edges"`
	Cycles            [][]string        `json:"cycles"`
	CycleCount        int               `json:"cycle_count"`
	HighCoupling      []string          `json:"high_coupling"`
	HighCouplingCount int               `json:"high_coupling_count"`
	FanInOut          []FanInOut        `json:"fan_in_out"`
}

type HotspotFile struct {
	FilePath      string  `json:"file_path"`
	Modifications int     `json:"modifications"`
	LinesChanged  int     `json:"lines_changed"`
	Churn         float64 `json:"churn"`
	HighComplexity bool   `json:"high_complexity"`
}

type GitHotspotReport struct {
	TimeRange         string        `json:"time_range"`
	TotalCommits      int           `json:"total_commits"`
	TopHotspots       []HotspotFile `json:"top_hotspots"`
	HighPriorityFiles []string      `json:"high_priority_files"`
}

type Contributor struct {
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	AddedLines   int     `json:"added_lines"`
	CommitCount  int     `json:"commit_count"`
	ActiveFiles  int     `json:"active_files"`
	Contribution float64 `json:"contribution"`
}

type ContributorReport struct {
	Contributors []Contributor `json:"contributors"`
	BusFactor    int           `json:"bus_factor"`
	TotalLines   int           `json:"total_lines"`
}

type TechDebtBreakdown struct {
	Complexity  float64 `json:"complexity"`
	Duplication float64 `json:"duplication"`
	Dependency  float64 `json:"dependency"`
	Hotspots    float64 `json:"hotspots"`
}

type TechDebtReport struct {
	Score      float64          `json:"score"`
	Grade      string           `json:"grade"`
	GradeColor string           `json:"-"`
	Breakdown  TechDebtBreakdown `json:"breakdown"`
}

type TrendMonth struct {
	Month            string  `json:"month"`
	CodeLines        int     `json:"code_lines"`
	Growth           float64 `json:"growth"`
	AvgComplexity    float64 `json:"avg_complexity"`
	ComplexityGrowth float64 `json:"complexity_growth"`
	DuplicationRate  float64 `json:"duplication_rate"`
	DuplicationGrowth float64 `json:"duplication_growth"`
}

type TrendReport struct {
	Months []TrendMonth `json:"months"`
}
