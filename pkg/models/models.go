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
	Diff          string
	DiffCommit1   string
	DiffCommit2   string
	RulesFile     string
}

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type RuleStatus string

const (
	RuleStatusPassed RuleStatus = "passed"
	RuleStatusFailed RuleStatus = "failed"
	RuleStatusSkipped RuleStatus = "skipped"
)

type LogicOperator string

const (
	LogicAND LogicOperator = "AND"
	LogicOR  LogicOperator = "OR"
)

type Rule struct {
	Name      string   `yaml:"name" json:"name"`
	Condition string   `yaml:"condition" json:"condition"`
	Severity  Severity `yaml:"severity" json:"severity"`
	Message   string   `yaml:"message,omitempty" json:"message,omitempty"`
}

type RuleGroup struct {
	Name     string        `yaml:"name" json:"name"`
	Logic    LogicOperator `yaml:"logic,omitempty" json:"logic,omitempty"`
	Rules    []Rule        `yaml:"rules" json:"rules"`
}

type RulesConfig struct {
	Extends string      `yaml:"extends,omitempty" json:"extends,omitempty"`
	Groups  []RuleGroup `yaml:"groups" json:"groups"`
}

type RuleResult struct {
	RuleName   string     `json:"rule_name"`
	GroupName  string     `json:"group_name"`
	Status     RuleStatus `json:"status"`
	Severity   Severity   `json:"severity"`
	Message    string     `json:"message,omitempty"`
	Actual     string     `json:"actual,omitempty"`
	SkipReason string     `json:"skip_reason,omitempty"`
}

type GroupResult struct {
	GroupName string       `json:"group_name"`
	Logic     LogicOperator `json:"logic"`
	Passed    bool         `json:"passed"`
	Results   []RuleResult `json:"results"`
}

type CustomRulesResult struct {
	Enabled      bool          `json:"enabled"`
	RulesFile    string        `json:"rules_file,omitempty"`
	TotalRules   int           `json:"total_rules"`
	PassedCount  int           `json:"passed_count"`
	FailedCount  int           `json:"failed_count"`
	SkippedCount int           `json:"skipped_count"`
	HasErrors    bool          `json:"has_errors"`
	Groups       []GroupResult `json:"groups,omitempty"`
	ParseError   string        `json:"parse_error,omitempty"`
}

type Report struct {
	RepoPath      string              `json:"repo_path"`
	GeneratedAt   time.Time           `json:"generated_at"`
	Metrics       *MetricsReport      `json:"metrics,omitempty"`
	Complexity    *ComplexityReport   `json:"complexity,omitempty"`
	Duplication   *DuplicationReport  `json:"duplication,omitempty"`
	Dependency    *DependencyReport   `json:"dependency,omitempty"`
	GitHotspots   *GitHotspotReport   `json:"git_hotspots,omitempty"`
	Contributors  *ContributorReport  `json:"contributors,omitempty"`
	TechDebt      *TechDebtReport     `json:"tech_debt,omitempty"`
	Trend         *TrendReport        `json:"trend,omitempty"`
	QualityGates  *QualityGateResult  `json:"quality_gates,omitempty"`
	CustomRules   *CustomRulesResult  `json:"custom_rules,omitempty"`
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

type ChangedFile struct {
	FilePath   string `json:"file_path"`
	ChangeType string `json:"change_type"`
	OldPath    string `json:"old_path,omitempty"`
}

type FileComplexityDiff struct {
	FilePath      string             `json:"file_path"`
	OldComplexity int                `json:"old_complexity"`
	NewComplexity int                `json:"new_complexity"`
	Diff          int                `json:"diff"`
	OldFunctions  []FunctionComplexity `json:"old_functions,omitempty"`
	NewFunctions  []FunctionComplexity `json:"new_functions,omitempty"`
	NewHighRisk   []FunctionComplexity `json:"new_high_risk,omitempty"`
}

type ComplexityDiffReport struct {
	TotalDiff        int                  `json:"total_diff"`
	ChangedFiles     int                  `json:"changed_files"`
	ImprovedFiles    int                  `json:"improved_files"`
	DegradedFiles    int                  `json:"degraded_files"`
	FileDiffs        []FileComplexityDiff `json:"file_diffs"`
	NewHighRiskCount int                  `json:"new_high_risk_count"`
}

type DuplicationDiffReport struct {
	NewDuplicationRate   float64           `json:"new_duplication_rate"`
	NewTotalTokens       int               `json:"new_total_tokens"`
	NewDuplicateTokens   int               `json:"new_duplicate_tokens"`
	NewBlockCount        int               `json:"new_block_count"`
	NewTopDuplicates     []DuplicateBlock  `json:"new_top_duplicates"`
}

type DependencyDiffReport struct {
	AddedEdges      []DependencyEdge `json:"added_edges"`
	RemovedEdges    []DependencyEdge `json:"removed_edges"`
	NewCycles       [][]string       `json:"new_cycles"`
	NewCycleCount   int              `json:"new_cycle_count"`
}

type IncrementalReport struct {
	RepoPath      string                  `json:"repo_path"`
	GeneratedAt   time.Time               `json:"generated_at"`
	Commit1       string                  `json:"commit1"`
	Commit2       string                  `json:"commit2"`
	ChangedFiles  []ChangedFile           `json:"changed_files"`
	Complexity    *ComplexityDiffReport   `json:"complexity,omitempty"`
	Duplication   *DuplicationDiffReport  `json:"duplication,omitempty"`
	Dependency    *DependencyDiffReport   `json:"dependency,omitempty"`
	QualityGates  *IncrementalGateResult  `json:"quality_gates,omitempty"`
}

type IncrementalGateResult struct {
	Passed     bool     `json:"passed"`
	Violations []string `json:"violations,omitempty"`
}
