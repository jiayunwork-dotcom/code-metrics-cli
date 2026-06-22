package duplication

import (
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/code-metrics/cli/pkg/models"
	"github.com/code-metrics/cli/pkg/utils"
)

type Token struct {
	Type  string
	Value string
	Line  int
}

type tokenizedFile struct {
	Path       string
	Tokens     []Token
	TokenHashes []uint64
}

type duplicateMatch struct {
	hash      uint64
	positions []position
}

type position struct {
	fileIdx int
	start   int
	end     int
	line    int
}

type tokenKey struct {
	fileIdx int
	idx     int
}

func Analyze(files []string, repoPath string, minTokenLen int, jobs int) *models.DuplicationReport {
	if len(files) == 0 {
		return &models.DuplicationReport{}
	}

	if minTokenLen <= 0 {
		minTokenLen = 50
	}

	pool := utils.NewWorkerPool(jobs)
	defer pool.Close()

	mu := sync.Mutex{}
	var tokenizedFiles []tokenizedFile
	totalTokens := 0

	for _, file := range files {
		file := file
		pool.Submit(func() {
			tokens := tokenizeFile(file)
			if len(tokens) == 0 {
				return
			}

			hashes := make([]uint64, len(tokens))
			for i, t := range tokens {
				hashes[i] = hashToken(t)
			}

			mu.Lock()
			tokenizedFiles = append(tokenizedFiles, tokenizedFile{
				Path:       file,
				Tokens:     tokens,
				TokenHashes: hashes,
			})
			totalTokens += len(tokens)
			mu.Unlock()
		})
	}

	pool.Wait()

	if totalTokens == 0 {
		return &models.DuplicationReport{}
	}

	sort.Slice(tokenizedFiles, func(i, j int) bool {
		return tokenizedFiles[i].Path < tokenizedFiles[j].Path
	})

	fileIndexMap := make(map[string]int)
	for i, tf := range tokenizedFiles {
		fileIndexMap[tf.Path] = i
	}

	matches := findDuplicates(tokenizedFiles, minTokenLen)

	duplicateTokens := 0
	marked := make(map[tokenKey]bool)

	for _, m := range matches {
		for _, pos := range m.positions {
			for i := pos.start; i < pos.end; i++ {
				key := tokenKey{fileIdx: pos.fileIdx, idx: i}
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

	blocks := buildDuplicateBlocks(matches, tokenizedFiles, repoPath, minTokenLen)

	return &models.DuplicationReport{
		DuplicationRate: duplicationRate,
		TotalTokens:     totalTokens,
		DuplicateTokens: duplicateTokens,
		BlockCount:      len(blocks),
		TopDuplicates:   blocks,
	}
}

func tokenizeFile(path string) []Token {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lang := utils.GetLanguageByExt(path)
	return tokenize(string(content), lang)
}

func tokenize(code string, lang utils.Language) []Token {
	var tokens []Token
	lines := strings.Split(code, "\n")

	for lineNum, line := range lines {
		line = stripLineComments(line, lang)
		if strings.TrimSpace(line) == "" {
			continue
		}

		lineTokens := tokenizeLine(line, lineNum+1)
		tokens = append(tokens, lineTokens...)
	}

	return tokens
}

func stripLineComments(line string, lang utils.Language) string {
	switch lang {
	case utils.LangPython:
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
	default:
		if idx := findLineCommentStart(line); idx >= 0 {
			line = line[:idx]
		}
	}
	return line
}

func findLineCommentStart(line string) int {
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

func tokenizeLine(line string, lineNum int) []Token {
	var tokens []Token
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
				tokens = append(tokens, Token{Type: "STRING", Value: "__STR__", Line: lineNum})
				current.Reset()
				inSingle = false
			}
			continue
		}

		if inDouble {
			current.WriteRune(ch)
			if ch == '"' {
				tokens = append(tokens, Token{Type: "STRING", Value: "__STR__", Line: lineNum})
				current.Reset()
				inDouble = false
			}
			continue
		}

		if inBacktick {
			current.WriteRune(ch)
			if ch == '`' {
				tokens = append(tokens, Token{Type: "STRING", Value: "__STR__", Line: lineNum})
				current.Reset()
				inBacktick = false
			}
			continue
		}

		if inChar {
			current.WriteRune(ch)
			if ch == '\'' {
				tokens = append(tokens, Token{Type: "CHAR", Value: "__CHAR__", Line: lineNum})
				current.Reset()
				inChar = false
			}
			continue
		}

		if unicode.IsSpace(ch) {
			if current.Len() > 0 {
				tokens = append(tokens, processWord(current.String(), lineNum))
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

		if isOperatorOrPunct(ch) {
			if current.Len() > 0 {
				tokens = append(tokens, processWord(current.String(), lineNum))
				current.Reset()
			}

			op := string(ch)
			if i+1 < len(runes) {
				twoChar := string(ch) + string(runes[i+1])
				if isTwoCharOp(twoChar) {
					op = twoChar
					i++
				}
			}
			tokens = append(tokens, Token{Type: "OP", Value: op, Line: lineNum})
			continue
		}

		current.WriteRune(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, processWord(current.String(), lineNum))
	}

	return tokens
}

func processWord(word string, lineNum int) Token {
	if isNumber(word) {
		return Token{Type: "NUMBER", Value: "__NUM__", Line: lineNum}
	}
	return Token{Type: "IDENT", Value: word, Line: lineNum}
}

func isNumber(s string) bool {
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

func isOperatorOrPunct(ch rune) bool {
	return strings.ContainsRune("+-*/%=<>!&|^~?.,;:()[]{}", ch)
}

func isTwoCharOp(s string) bool {
	return strings.Contains(
		"== != <= >= && || ++ -- += -= *= /= %= &= |= ^= << >> -> ::",
		s,
	)
}

func hashToken(t Token) uint64 {
	h := fnv.New64a()
	h.Write([]byte(t.Type + ":" + t.Value))
	return h.Sum64()
}

const (
	rollBase  = 1099511628211
	rollMod   = 18446744073709551557
)

func hashWindow(hashes []uint64, start, length int) uint64 {
	h := uint64(0)
	end := start + length
	if end > len(hashes) {
		end = len(hashes)
	}
	for i := start; i < end; i++ {
		h = (h * rollBase + hashes[i]) % rollMod
	}
	return h
}

func findDuplicates(files []tokenizedFile, minLen int) []duplicateMatch {
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
			h := hashWindow(tf.TokenHashes, i, windowLen)
			line := tf.Tokens[i].Line
			hashMap[h] = append(hashMap[h], occurrence{fileIdx, i, line})
		}
	}

	var matches []duplicateMatch
	used := make(map[tokenKey]bool)

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

		var validPositions []position
		for _, occ := range occurs {
			start := occ.pos
			end := occ.pos + windowLen
			if end > len(files[occ.fileIdx].TokenHashes) {
				end = len(files[occ.fileIdx].TokenHashes)
			}

			overlap := false
			for i := start; i < end; i++ {
				if used[tokenKey{fileIdx: occ.fileIdx, idx: i}] {
					overlap = true
					break
				}
			}
			if overlap {
				continue
			}

			validPositions = append(validPositions, position{
				fileIdx: occ.fileIdx,
				start:   start,
				end:     end,
				line:    occ.line,
			})
		}

		if len(validPositions) >= 2 {
			for _, pos := range validPositions {
				for i := pos.start; i < pos.end; i++ {
					used[tokenKey{fileIdx: pos.fileIdx, idx: i}] = true
				}
			}

			matches = append(matches, duplicateMatch{
				hash:      h,
				positions: validPositions,
			})
		}
	}

	return matches
}

func buildDuplicateBlocks(matches []duplicateMatch, files []tokenizedFile, repoPath string, minLen int) []models.DuplicateBlock {
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
