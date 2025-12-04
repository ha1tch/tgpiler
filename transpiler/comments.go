package transpiler

import (
	"fmt"
	"regexp"
	"strings"
)

// commentInfo holds a comment and its location.
type commentInfo struct {
	text      string // The comment text (without -- or /* */)
	line      int    // Line number (1-indexed)
	isBlock   bool   // True for /* */ comments
	isTrailing bool  // True if comment is on same line after code
}

// commentExtractor extracts comments from T-SQL source.
type commentExtractor struct {
	comments []commentInfo
	source   string
	lines    []string
}

func newCommentExtractor(source string) *commentExtractor {
	ce := &commentExtractor{
		source: source,
		lines:  strings.Split(source, "\n"),
	}
	ce.extract()
	return ce
}

func (ce *commentExtractor) extract() {
	for lineNum, line := range ce.lines {
		ce.extractFromLine(line, lineNum+1) // 1-indexed
	}
}

func (ce *commentExtractor) extractFromLine(line string, lineNum int) {
	// Check for line comment
	if idx := strings.Index(line, "--"); idx != -1 {
		// Check if it's inside a string (simplified check)
		beforeComment := line[:idx]
		if !ce.isInsideString(beforeComment) {
			commentText := strings.TrimSpace(line[idx+2:])
			isTrailing := strings.TrimSpace(beforeComment) != ""
			ce.comments = append(ce.comments, commentInfo{
				text:       commentText,
				line:       lineNum,
				isBlock:    false,
				isTrailing: isTrailing,
			})
		}
	}

	// Check for block comment start on this line
	if idx := strings.Index(line, "/*"); idx != -1 {
		beforeComment := line[:idx]
		if !ce.isInsideString(beforeComment) {
			// Find the end - could span multiple lines
			ce.extractBlockComment(lineNum, idx)
		}
	}
}

func (ce *commentExtractor) extractBlockComment(startLine, startCol int) {
	// Simple block comment extraction - find matching */
	text := ce.source
	
	// Find position in source
	pos := 0
	for i := 0; i < startLine-1; i++ {
		pos += len(ce.lines[i]) + 1 // +1 for newline
	}
	pos += startCol
	
	// Find /* and */
	startIdx := strings.Index(text[pos:], "/*")
	if startIdx == -1 {
		return
	}
	startIdx += pos
	
	endIdx := strings.Index(text[startIdx+2:], "*/")
	if endIdx == -1 {
		return
	}
	endIdx += startIdx + 2
	
	commentText := strings.TrimSpace(text[startIdx+2 : endIdx])
	
	// Check if there's code before the comment on the same line
	beforeComment := ce.lines[startLine-1][:startCol]
	isTrailing := strings.TrimSpace(beforeComment) != ""
	
	ce.comments = append(ce.comments, commentInfo{
		text:       commentText,
		line:       startLine,
		isBlock:    true,
		isTrailing: isTrailing,
	})
}

func (ce *commentExtractor) isInsideString(s string) bool {
	// Count unescaped single quotes
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			// Check for escaped quote ('')
			if i+1 < len(s) && s[i+1] == '\'' {
				i++ // Skip the escaped quote
			} else {
				count++
			}
		}
	}
	return count%2 == 1
}

// statementMatcher helps associate comments with statements.
type statementMatcher struct {
	patterns map[string]*regexp.Regexp
}

func newStatementMatcher() *statementMatcher {
	sm := &statementMatcher{
		patterns: make(map[string]*regexp.Regexp),
	}
	
	// Patterns to identify statement types - case insensitive
	patterns := map[string]string{
		"CREATE_PROC":   `(?i)^\s*CREATE\s+(PROCEDURE|PROC)\s+`,
		"DECLARE":       `(?i)^\s*DECLARE\s+@`,
		"SET":           `(?i)^\s*SET\s+@`,
		"IF":            `(?i)^\s*IF\s+`,
		"ELSE":          `(?i)^\s*ELSE\s*`,
		"WHILE":         `(?i)^\s*WHILE\s+`,
		"BEGIN":         `(?i)^\s*BEGIN\s*$`,
		"END":           `(?i)^\s*END\s*`,
		"RETURN":        `(?i)^\s*RETURN\b`,
		"PRINT":         `(?i)^\s*PRINT\s+`,
		"BEGIN_TRY":     `(?i)^\s*BEGIN\s+TRY`,
		"END_TRY":       `(?i)^\s*END\s+TRY`,
		"BEGIN_CATCH":   `(?i)^\s*BEGIN\s+CATCH`,
		"END_CATCH":     `(?i)^\s*END\s+CATCH`,
		"BREAK":         `(?i)^\s*BREAK\s*`,
		"CONTINUE":      `(?i)^\s*CONTINUE\s*`,
		"SELECT":        `(?i)^\s*SELECT\s+`,
		"INSERT":        `(?i)^\s*INSERT\s+`,
		"UPDATE":        `(?i)^\s*UPDATE\s+`,
		"DELETE":        `(?i)^\s*DELETE\s+`,
		"EXEC":          `(?i)^\s*(EXEC|EXECUTE)\s+`,
	}
	
	for name, pattern := range patterns {
		sm.patterns[name] = regexp.MustCompile(pattern)
	}
	
	return sm
}

func (sm *statementMatcher) identify(line string) string {
	for name, pattern := range sm.patterns {
		if pattern.MatchString(line) {
			return name
		}
	}
	return ""
}

// commentMap associates comments with source constructs.
type commentMap struct {
	// Leading comments indexed by line number of the statement they precede
	leading map[int][]string
	// Trailing comments indexed by line number
	trailing map[int]string
	// Procedure-level comments (before CREATE PROCEDURE)
	procComments map[string][]string
}

func buildCommentMap(source string) *commentMap {
	ce := newCommentExtractor(source)
	sm := newStatementMatcher()
	lines := strings.Split(source, "\n")
	
	cm := &commentMap{
		leading:      make(map[int][]string),
		trailing:     make(map[int]string),
		procComments: make(map[string][]string),
	}
	
	// Track pending comments (comments waiting for a statement)
	var pendingComments []string
	
	commentIdx := 0
	for lineNum := 1; lineNum <= len(lines); lineNum++ {
		line := lines[lineNum-1]
		
		// Collect any comments on this line
		for commentIdx < len(ce.comments) && ce.comments[commentIdx].line == lineNum {
			c := ce.comments[commentIdx]
			if c.isTrailing {
				cm.trailing[lineNum] = c.text
			} else {
				pendingComments = append(pendingComments, c.text)
			}
			commentIdx++
		}
		
		// Check if this line has a statement
		stmtType := sm.identify(line)
		if stmtType != "" && len(pendingComments) > 0 {
			cm.leading[lineNum] = pendingComments
			pendingComments = nil
		}
	}
	
	return cm
}

// convertComment converts a T-SQL comment to Go format.
func convertComment(text string, isBlock bool) string {
	if isBlock {
		// Multi-line block comments
		if strings.Contains(text, "\n") {
			lines := strings.Split(text, "\n")
			var result []string
			for _, line := range lines {
				result = append(result, "// "+strings.TrimSpace(line))
			}
			return strings.Join(result, "\n")
		}
		return "// " + text
	}
	return "// " + text
}

// commentIndex maps statement signatures to their leading comments.
type commentIndex struct {
	comments map[string][]string // signature -> comments
	used     map[string]bool     // track which signatures have been consumed
}

// buildCommentIndex scans source and builds an index of comments by statement signature.
func buildCommentIndex(source string) *commentIndex {
	ci := &commentIndex{
		comments: make(map[string][]string),
		used:     make(map[string]bool),
	}

	lines := strings.Split(source, "\n")
	var pendingComments []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for line comment
		if strings.HasPrefix(trimmed, "--") {
			commentText := strings.TrimSpace(trimmed[2:])
			if commentText != "" {
				pendingComments = append(pendingComments, commentText)
			}
			continue
		}

		// Check for block comment (single line only for simplicity)
		if strings.HasPrefix(trimmed, "/*") {
			endIdx := strings.Index(trimmed, "*/")
			if endIdx != -1 {
				commentText := strings.TrimSpace(trimmed[2:endIdx])
				if commentText != "" {
					pendingComments = append(pendingComments, commentText)
				}
			}
			continue
		}

		// Skip empty lines but preserve pending comments
		if trimmed == "" {
			continue
		}

		// Try to extract a signature from this line
		sig := extractSignature(trimmed)
		if sig != "" && len(pendingComments) > 0 {
			// Make signature unique by appending a counter if it already exists
			baseSig := sig
			counter := 1
			for ci.comments[sig] != nil {
				sig = fmt.Sprintf("%s#%d", baseSig, counter)
				counter++
			}
			ci.comments[sig] = append(ci.comments[sig], pendingComments...)
			pendingComments = nil
		} else if sig == "" && len(pendingComments) > 0 {
			// Non-empty line but no recognized signature - comments may be for a sub-part
			// Keep them pending
		}

		// Also handle trailing comments on the same line
		if idx := strings.Index(trimmed, "--"); idx > 0 {
			trailingSig := extractSignature(trimmed[:idx])
			if trailingSig != "" {
				trailingComment := strings.TrimSpace(trimmed[idx+2:])
				if trailingComment != "" {
					ci.comments[trailingSig+"#trailing"] = []string{trailingComment}
				}
			}
		}
	}

	return ci
}

// extractSignature extracts a lookup key from a T-SQL statement line.
func extractSignature(line string) string {
	line = strings.TrimSpace(line)
	upper := strings.ToUpper(line)

	// CREATE PROCEDURE dbo.Name -> "PROC:Name"
	if strings.HasPrefix(upper, "CREATE PROC") {
		re := regexp.MustCompile(`(?i)CREATE\s+(?:PROCEDURE|PROC)\s+(?:\w+\.)*(\w+)`)
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			return "PROC:" + strings.ToLower(m[1])
		}
	}

	// DECLARE @Name -> "DECLARE:name"
	if strings.HasPrefix(upper, "DECLARE") {
		re := regexp.MustCompile(`(?i)DECLARE\s+@(\w+)`)
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			return "DECLARE:" + strings.ToLower(m[1])
		}
	}

	// SET @Name = -> "SET:name"
	if strings.HasPrefix(upper, "SET") && strings.Contains(line, "@") {
		re := regexp.MustCompile(`(?i)SET\s+@(\w+)`)
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			return "SET:" + strings.ToLower(m[1])
		}
	}

	// IF -> "IF:condition_start" (use first few chars of condition)
	if strings.HasPrefix(upper, "IF ") || strings.HasPrefix(upper, "IF(") {
		// Extract a short identifier from the condition
		re := regexp.MustCompile(`(?i)IF\s*\(?@?(\w+)`)
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			return "IF:" + strings.ToLower(m[1])
		}
		return "IF"
	}

	// ELSE IF -> "ELSEIF:condition_start"
	if strings.HasPrefix(upper, "ELSE IF") {
		re := regexp.MustCompile(`(?i)ELSE\s+IF\s*\(?@?(\w+)`)
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			return "ELSEIF:" + strings.ToLower(m[1])
		}
		return "ELSEIF"
	}

	// ELSE -> "ELSE"
	if upper == "ELSE" || strings.HasPrefix(upper, "ELSE ") {
		return "ELSE"
	}

	// WHILE -> "WHILE:condition_start"
	if strings.HasPrefix(upper, "WHILE") {
		re := regexp.MustCompile(`(?i)WHILE\s*\(?@?(\w+)`)
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			return "WHILE:" + strings.ToLower(m[1])
		}
		return "WHILE"
	}

	// BEGIN TRY -> "BEGINTRY"
	if strings.Contains(upper, "BEGIN") && strings.Contains(upper, "TRY") {
		return "BEGINTRY"
	}

	// BEGIN CATCH -> "BEGINCATCH"
	if strings.Contains(upper, "BEGIN") && strings.Contains(upper, "CATCH") {
		return "BEGINCATCH"
	}

	// RETURN -> "RETURN"
	if strings.HasPrefix(upper, "RETURN") {
		return "RETURN"
	}

	// PRINT -> "PRINT:content_start"
	if strings.HasPrefix(upper, "PRINT") {
		re := regexp.MustCompile(`(?i)PRINT\s+['"]?(\w+)`)
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			return "PRINT:" + strings.ToLower(m[1])
		}
		return "PRINT"
	}

	// BEGIN/END blocks - less specific
	if upper == "BEGIN" {
		return "BEGIN"
	}
	if upper == "END" {
		return "END"
	}

	return ""
}

// lookup returns comments for a given signature and marks them as used.
func (ci *commentIndex) lookup(sig string) []string {
	if ci == nil || ci.comments == nil {
		return nil
	}
	
	// Try exact match first
	if comments, ok := ci.comments[sig]; ok && !ci.used[sig] {
		ci.used[sig] = true
		return comments
	}
	
	// Try numbered variants
	for i := 1; i < 100; i++ {
		numberedSig := fmt.Sprintf("%s#%d", sig, i)
		if comments, ok := ci.comments[numberedSig]; ok && !ci.used[numberedSig] {
			ci.used[numberedSig] = true
			return comments
		}
	}
	
	return nil
}

// lookupTrailing returns trailing comment for a signature.
func (ci *commentIndex) lookupTrailing(sig string) string {
	if ci == nil || ci.comments == nil {
		return ""
	}
	trailingSig := sig + "#trailing"
	if comments := ci.comments[trailingSig]; len(comments) > 0 && !ci.used[trailingSig] {
		ci.used[trailingSig] = true
		return comments[0]
	}
	return ""
}
