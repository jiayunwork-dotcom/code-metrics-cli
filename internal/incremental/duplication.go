package incremental

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/code-metrics/cli/internal/git"
	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

type IncToken struct {
	Type  string
	Value string
	Line  int
}

type IncTokenizedFile struct {
	Path       string
	Tokens     []IncToken
	TokenHashes []uint64
}

type IncDuplicateMatch struct {
	hash      uint64
	positions []IncPosition
}

type IncPosition struct {
	fileIdx int
	start   int
	end     int
	line    int
}

type IncTokenKey struct {
	fileIdx int
	idx     int
}

func AnalyzeDuplicationDiff(changedFiles []models.ChangedFile, opts *models.AnalyzerOptions) *models.DuplicationDiffReport {
	if len(changedFiles) == 0 {
		return &models.DuplicationDiffReport{}
	}

	minTokenLen := opts.MinTokenLen
	if minTokenLen <= 0 {
		minTokenLen = 50
	}

	oldTokenizedFiles, newTokenizedFiles, totalTokens := tokenizeChangedFiles(changedFiles, opts)

	if totalTokens == 0 {
		return &models.DuplicationDiffReport{}
	}

	oldMatches := findDuplicates(oldTokenizedFiles, minTokenLen)
	newMatches := findDuplicates(newTokenizedFiles, minTokenLen)

	newDuplicates := filterNewDuplicates(oldMatches, newMatches, oldTokenizedFiles, newTokenizedFiles)

	duplicateTokens := 0
	marked := make(map[IncTokenKey]bool)

	for _, m := range newDuplicates {
		for _, pos := range m.positions {
			for i := pos.start; i < pos.end; i++ {
				key := IncTokenKey{fileIdx: pos.fileIdx, idx: i}
				if !marked[key] {
					marked[key] = true
					duplicateTokens++
				}
			}
		}
	}

	duplicationRate := 0.0
	if totalTokens > 0 {
		duplicationRate = utils.RoundFloat(float64(duplicateTokens)/float64(totalTokens)*100, 1)
	}

	blocks := buildIncDuplicateBlocks(newDuplicates, newTokenizedFiles, opts.RepoPath, minTokenLen)

	return &models.DuplicationDiffReport{
		NewDuplicationRate: duplicationRate,
		NewTotalTokens:     totalTokens,
		NewDuplicateTokens: duplicateTokens,
		NewBlockCount:      len(blocks),
		NewTopDuplicates:   blocks,
	}
}

func tokenizeChangedFiles(changedFiles []models.ChangedFile, opts *models.AnalyzerOptions) ([]IncTokenizedFile, []IncTokenizedFile, int) {
	pool := utils.NewWorkerPool(opts.Jobs)
	defer pool.Close()

	mu := sync.Mutex{}
	var oldTokenized []IncTokenizedFile
	var newTokenized []IncTokenizedFile
	totalTokens := 0

	for _, cf := range changedFiles {
		cf := cf
		pool.Submit(func() {
			lang := utils.GetLanguageByExt(cf.FilePath)
			oldPath := cf.FilePath
			if cf.OldPath != "" {
				oldPath = cf.OldPath
			}

			if cf.ChangeType != "added" {
				oldContent, err := git.GetFileContent(opts.RepoPath, opts.DiffCommit1, oldPath)
				if err == nil && oldContent != "" {
					oldTokens := tokenizeContent(oldContent, lang)
					if len(oldTokens) > 0 {
						oldHashes := make([]uint64, len(oldTokens))
						for i, t := range oldTokens {
							oldHashes[i] = hashIncToken(t)
						}
						mu.Lock()
						oldTokenized = append(oldTokenized, IncTokenizedFile{
							Path:       cf.FilePath,
							Tokens:     oldTokens,
							TokenHashes: oldHashes,
						})
						mu.Unlock()
					}
				}
			}

			newContent, err := git.GetFileContent(opts.RepoPath, opts.DiffCommit2, cf.FilePath)
			if err == nil && newContent != "" {
				newTokens := tokenizeContent(newContent, lang)
				if len(newTokens) > 0 {
					newHashes := make([]uint64, len(newTokens))
					for i, t := range newTokens {
						newHashes[i] = hashIncToken(t)
					}
					mu.Lock()
					newTokenized = append(newTokenized, IncTokenizedFile{
						Path:       cf.FilePath,
						Tokens:     newTokens,
						TokenHashes: newHashes,
					})
					totalTokens += len(newTokens)
					mu.Unlock()
				}
			}
		})
	}

	pool.Wait()

	sort.Slice(oldTokenized, func(i, j int) bool {
		return oldTokenized[i].Path < oldTokenized[j].Path
	})
	sort.Slice(newTokenized, func(i, j int) bool {
		return newTokenized[i].Path < newTokenized[j].Path
	})

	return oldTokenized, newTokenized, totalTokens
}

func tokenizeContent(code string, lang utils.Language) []IncToken {
	var tokens []IncToken
	lines := strings.Split(code, "\n")

	for lineNum, line := range lines {
		line = stripIncLineComments(line, lang)
		if strings.TrimSpace(line) == "" {
			continue
		}

		lineTokens := tokenizeIncLine(line, lineNum+1)
		tokens = append(tokens, lineTokens...)
	}

	return tokens
}

func stripIncLineComments(line string, lang utils.Language) string {
	switch lang {
	case utils.LangPython:
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
	default:
		if idx := findIncLineCommentStart(line); idx >= 0 {
			line = line[:idx]
		}
	}
	return line
}

func findIncLineCommentStart(line string) int {
	inSingle := false
	inDouble := false
	inBacktick := false
	runes := []rune(line)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == '\\' && i+1 < len(runes) {
			i++
			continue
		}
		if inSingle {
			if ch == '\'' {
				inSingle = false
			}
		} else if inDouble {
			if ch == '"' {
				inDouble = false
			}
		} else if inBacktick {
			if ch == '`' {
				inBacktick = false
			}
		} else {
			if ch == '\'' {
				inSingle = true
			} else if ch == '"' {
				inDouble = true
			} else if ch == '`' {
				inBacktick = true
			} else if ch == '/' && i+1 < len(runes) && runes[i+1] == '/' {
				return i
			}
		}
	}
	return -1
}

func tokenizeIncLine(line string, lineNum int) []IncToken {
	var tokens []IncToken
	var current strings.Builder
	inSingle := false
	inDouble := false
	inBacktick := false
	inChar := false
	runes := []rune(line)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		if ch == '\\' && i+1 < len(runes) && (inSingle || inDouble || inBacktick || inChar) {
			current.WriteRune(ch)
			current.WriteRune(runes[i+1])
			i++
			continue
		}

		if inSingle {
			current.WriteRune(ch)
			if ch == '\'' {
				tokens = append(tokens, IncToken{Type: "STRING", Value: "__STR__", Line: lineNum})
				current.Reset()
				inSingle = false
			}
			continue
		}

		if inDouble {
			current.WriteRune(ch)
			if ch == '"' {
				tokens = append(tokens, IncToken{Type: "STRING", Value: "__STR__", Line: lineNum})
				current.Reset()
				inDouble = false
			}
			continue
		}

		if inBacktick {
			current.WriteRune(ch)
			if ch == '`' {
				tokens = append(tokens, IncToken{Type: "STRING", Value: "__STR__", Line: lineNum})
				current.Reset()
				inBacktick = false
			}
			continue
		}

		if inChar {
			current.WriteRune(ch)
			if ch == '\'' {
				tokens = append(tokens, IncToken{Type: "CHAR", Value: "__CHAR__", Line: lineNum})
				current.Reset()
				inChar = false
			}
			continue
		}

		if unicode.IsSpace(ch) {
			if current.Len() > 0 {
				tokens = append(tokens, processIncWord(current.String(), lineNum))
				current.Reset()
			}
			continue
		}

		if ch == '\'' && i+2 < len(runes) && runes[i+2] == '\'' {
			inChar = true
			current.WriteRune(ch)
			continue
		}

		if ch == '\'' {
			inSingle = true
			current.WriteRune(ch)
			continue
		}

		if ch == '"' {
			inDouble = true
			current.WriteRune(ch)
			continue
		}

		if ch == '`' {
			inBacktick = true
			current.WriteRune(ch)
			continue
		}

		if isIncOperatorOrPunct(ch) {
			if current.Len() > 0 {
				tokens = append(tokens, processIncWord(current.String(), lineNum))
				current.Reset()
			}

			op := string(ch)
			if i+1 < len(runes) {
				twoChar := string(ch) + string(runes[i+1])
				if isIncTwoCharOp(twoChar) {
					op = twoChar
					i++
				}
			}
			tokens = append(tokens, IncToken{Type: "OP", Value: op, Line: lineNum})
			continue
		}

		current.WriteRune(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, processIncWord(current.String(), lineNum))
	}

	return tokens
}

func processIncWord(word string, lineNum int) IncToken {
	if isIncNumber(word) {
		return IncToken{Type: "NUMBER", Value: "__NUM__", Line: lineNum}
	}
	return IncToken{Type: "IDENT", Value: word, Line: lineNum}
}

func isIncNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	hasDot := false
	hasDigits := false
	for i, ch := range s {
		if i == 0 && (ch == '+' || ch == '-') && len(s) > 1 {
			continue
		}
		if ch == '.' && !hasDot {
			hasDot = true
			continue
		}
		if ch >= '0' && ch <= '9' {
			hasDigits = true
			continue
		}
		if (ch == 'e' || ch == 'E') && hasDigits {
			return true
		}
		if (ch == 'x' || ch == 'X') && i == 1 && s[0] == '0' {
			return true
		}
		if (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			if len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
				continue
			}
		}
		return false
	}
	return hasDigits
}

func isIncOperatorOrPunct(ch rune) bool {
	return strings.ContainsRune("+-*/%=<>!&|^~?.,;:()[]{}", ch)
}

func isIncTwoCharOp(s string) bool {
	return strings.Contains(
		"== != <= >= && || ++ -- += -= *= /= %= &= |= ^= << >> -> ::",
		s,
	)
}

func hashIncToken(t IncToken) uint64 {
	h := fnv.New64a()
	h.Write([]byte(t.Type + ":" + t.Value))
	return h.Sum64()
}

const (
	incRollBase = 1099511628211
	incRollMod  = 18446744073709551557
)

func hashIncWindow(hashes []uint64, start, length int) uint64 {
	h := uint64(0)
	end := start + length
	if end > len(hashes) {
		end = len(hashes)
	}
	for i := start; i < end; i++ {
		h = (h * incRollBase + hashes[i]) % incRollMod
	}
	return h
}

func findDuplicates(files []IncTokenizedFile, minLen int) []IncDuplicateMatch {
	type occurrence struct {
		fileIdx int
		pos     int
		line    int
	}

	hashMap := make(map[uint64][]occurrence)
	windowLen := minLen
	stepSize := minLen / 2
	if stepSize < 1 {
		stepSize = 1
	}

	for fileIdx, tf := range files {
		if len(tf.TokenHashes) < windowLen {
			continue
		}

		for i := 0; i <= len(tf.TokenHashes)-windowLen; i += stepSize {
			h := hashIncWindow(tf.TokenHashes, i, windowLen)
			line := tf.Tokens[i].Line
			hashMap[h] = append(hashMap[h], occurrence{fileIdx, i, line})
		}
	}

	var matches []IncDuplicateMatch
	used := make(map[IncTokenKey]bool)

	type hashOccurrences struct {
		h      uint64
		occurs []occurrence
	}

	var sorted []hashOccurrences
	for h, occurs := range hashMap {
		if len(occurs) >= 2 {
			sorted = append(sorted, hashOccurrences{h, occurs})
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].occurs) > len(sorted[j].occurs)
	})

	for _, ho := range sorted {
		h := ho.h
		occurs := ho.occurs

		var validPositions []IncPosition
		for _, occ := range occurs {
			start := occ.pos
			end := occ.pos + windowLen
			if end > len(files[occ.fileIdx].TokenHashes) {
				end = len(files[occ.fileIdx].TokenHashes)
			}

			overlap := false
			for i := start; i < end; i++ {
				if used[IncTokenKey{fileIdx: occ.fileIdx, idx: i}] {
					overlap = true
					break
				}
			}
			if overlap {
				continue
			}

			validPositions = append(validPositions, IncPosition{
				fileIdx: occ.fileIdx,
				start:   start,
				end:     end,
				line:    occ.line,
			})
		}

		if len(validPositions) >= 2 {
			for _, pos := range validPositions {
				for i := pos.start; i < pos.end; i++ {
					used[IncTokenKey{fileIdx: pos.fileIdx, idx: i}] = true
				}
			}

			matches = append(matches, IncDuplicateMatch{
				hash:      h,
				positions: validPositions,
			})
		}
	}

	return matches
}

func filterNewDuplicates(oldMatches, newMatches []IncDuplicateMatch, oldFiles, newFiles []IncTokenizedFile) []IncDuplicateMatch {
	oldHashes := make(map[uint64]bool)
	for _, m := range oldMatches {
		oldHashes[m.hash] = true
	}

	oldPathIndex := make(map[string]int)
	for i, f := range oldFiles {
		oldPathIndex[f.Path] = i
	}

	var newDuplicates []IncDuplicateMatch
	for _, m := range newMatches {
		if oldHashes[m.hash] {
			continue
		}

		hasNewOccurrence := false
		for _, pos := range m.positions {
			filePath := newFiles[pos.fileIdx].Path
			oldIdx, exists := oldPathIndex[filePath]
			if !exists {
				hasNewOccurrence = true
				break
			}

			posInOld := false
			for _, oldPos := range m.positions {
				if oldPos.fileIdx == oldIdx {
					posInOld = true
					break
				}
			}
			if !posInOld {
				hasNewOccurrence = true
				break
			}
		}

		if hasNewOccurrence {
			newDuplicates = append(newDuplicates, m)
		}
	}

	return newDuplicates
}

func buildIncDuplicateBlocks(matches []IncDuplicateMatch, files []IncTokenizedFile, repoPath string, minLen int) []models.DuplicateBlock {
	type blockKey struct {
		len       int
		positions string
	}

	blockMap := make(map[blockKey]*models.DuplicateBlock)

	for _, m := range matches {
		if len(m.positions) < 2 {
			continue
		}

		tokenLen := m.positions[0].end - m.positions[0].start
		isCrossFile := false
		fileSet := make(map[int]bool)

		var locations []models.DuplicateLocation
		for _, pos := range m.positions {
			fileSet[pos.fileIdx] = true
			filePath, _ := strings.CutPrefix(files[pos.fileIdx].Path, repoPath+"/")
			if filePath == files[pos.fileIdx].Path {
				filePath = files[pos.fileIdx].Path
			}

			startLine := pos.line
			endLine := startLine
			if pos.end < len(files[pos.fileIdx].Tokens) {
				endLine = files[pos.fileIdx].Tokens[pos.end-1].Line
			}

			locations = append(locations, models.DuplicateLocation{
				FilePath:  filePath,
				StartLine: startLine,
				EndLine:   endLine,
			})
		}

		if len(fileSet) > 1 {
			isCrossFile = true
		}

		sort.Slice(locations, func(i, j int) bool {
			if locations[i].FilePath == locations[j].FilePath {
				return locations[i].StartLine < locations[j].StartLine
			}
			return locations[i].FilePath < locations[j].FilePath
		})

		var posKey strings.Builder
		for _, loc := range locations {
			posKey.WriteString(fmt.Sprintf("%s:%d-%d;", loc.FilePath, loc.StartLine, loc.EndLine))
		}

		key := blockKey{len: tokenLen, positions: posKey.String()}
		if existing, ok := blockMap[key]; ok {
			existing.Occurrences++
		} else {
			blockMap[key] = &models.DuplicateBlock{
				TokenLength: tokenLen,
				Occurrences: len(locations),
				Locations:   locations,
				IsCrossFile: isCrossFile,
			}
		}
	}

	var blocks []models.DuplicateBlock
	for _, b := range blockMap {
		blocks = append(blocks, *b)
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].TokenLength*blocks[i].Occurrences > blocks[j].TokenLength*blocks[j].Occurrences
	})

	topN := 10
	if len(blocks) < topN {
		topN = len(blocks)
	}

	return blocks[:topN]
}
