package store

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Parser represents an input plugin that parses a file into a Source in-memory database.
type Parser interface {
	// Parse reads a file from filePath with fileContent and loads symbols into the provided source store.
	Parse(filePath string, fileContent []byte, source *Source) error
}

// Generator represents an output plugin that produces structured documentation from a Source in-memory database.
type Generator interface {
	// Generate converts all symbols and files in the source store and writes the output to the specified directory.
	Generate(source *Source, outputDir string) error
}

// SymbolType specifies the kind of symbol represented (e.g. struct, function, method, field).
type SymbolType string

const (
	// SymStruct represents a Go struct type declaration.
	SymStruct    SymbolType = "struct"
	// SymInterface represents a Go interface type declaration.
	SymInterface SymbolType = "interface"
	// SymFunction represents a Go global function declaration.
	SymFunction  SymbolType = "function"
	// SymMethod represents a struct method receiver declaration.
	SymMethod    SymbolType = "method"
	// SymField represents a Go struct field declaration.
	SymField     SymbolType = "field"
	// SymVariable represents a global or package-level variable declaration.
	SymVariable  SymbolType = "variable"
	// SymImport represents an import declaration.
	SymImport    SymbolType = "import"
)

// Symbol stores detailed metadata about a parsed code element.
type Symbol struct {
	// Name is the identifier name of the symbol.
	Name          string
	// Kind is the type of symbol (struct, function, method, field).
	Kind          SymbolType
	// File is the file path where the symbol is defined.
	File          string
	// Line is the 1-based line number where the symbol starts.
	Line          int
	// Doc contains the parsed and cleaned leading comment block text.
	Doc           string
	// Audience lists the targeted readership tags (e.g. API, INTERNAL, DEVELOPER, USER).
	Audience      []string
	// Compatibility lists the target system namespaces (e.g. C, RUST, JS).
	Compatibility []string
	// Package holds the package name where the symbol is defined.
	Package       string
	// Parent holds the name of the parent struct for fields and methods.
	Parent        string
	// Params holds the parameters of a function or method.
	Params        string
	// Returns holds the return type(s) of a function or method.
	Returns       string
	// Type holds the type of a field or variable.
	Type          string
	// Value holds the constant or default value of a variable/constant.
	Value         string
	// LineCount is the number of lines of a function/method.
	LineCount     int
	// Complexity is the estimated cognitive/cyclomatic complexity.
	Complexity    int
	// Relations stores types that this symbol inherits from, implements, or is composed of.
	Relations     []string
	// Coverage holds the statement coverage percentage if a coverage report is present.
	Coverage      *float64
}

// File represents a registered source file inside our parsed database.
type File struct {
	// Name is the relative file path.
	Name string
}

// CallRelation stores a caller-to-callee connection.
type CallRelation struct {
	// Caller is the name of the calling function or method (e.g. "GoParser.Parse").
	Caller string
	// Callee is the name of the called function or method (e.g. "Source.AddFile").
	Callee string
}

// Source serves as an in-memory normalized database of all parsed files and symbols.
type Source struct {
	// Files is the list of all registered source files.
	Files   []File
	// Symbols is the list of all parsed symbols.
	Symbols []Symbol
	// Calls is the list of all registered caller-to-callee relationships.
	Calls   []CallRelation

	indexesBuilt bool
	callersIndex map[string][]string
	calleesIndex map[string][]string
	callsSet     map[CallRelation]bool
}

func (s *Source) buildIndexes() {
	s.callersIndex = make(map[string][]string)
	s.calleesIndex = make(map[string][]string)

	addUnique := func(m map[string][]string, key string, val string) {
		for _, existing := range m[key] {
			if strings.EqualFold(existing, val) {
				return
			}
		}
		m[key] = append(m[key], val)
	}

	for _, c := range s.Calls {
		langCaller := s.getSymbolLanguage(c.Caller)
		langCallee := s.getSymbolLanguage(c.Callee)
		if !areLanguagesCompatible(langCaller, langCallee) {
			continue
		}

		calleeLower := strings.ToLower(c.Callee)
		callerLower := strings.ToLower(c.Caller)

		// Index for GetCallers (key is callee, value is caller)
		addUnique(s.callersIndex, calleeLower, c.Caller)
		for i, ch := range calleeLower {
			if ch == '.' && i < len(calleeLower)-1 {
				addUnique(s.callersIndex, calleeLower[i+1:], c.Caller)
			}
		}

		// Index for GetCallees (key is caller, value is callee)
		addUnique(s.calleesIndex, callerLower, c.Callee)
		for i, ch := range callerLower {
			if ch == '.' && i < len(callerLower)-1 {
				addUnique(s.calleesIndex, callerLower[i+1:], c.Callee)
			}
		}
	}
	s.indexesBuilt = true
}

func (s *Source) getSymbolLanguage(name string) string {
	nameLower := strings.ToLower(name)
	for _, sym := range s.Symbols {
		symLower := strings.ToLower(sym.Name)
		if symLower == nameLower || strings.HasSuffix(symLower, "."+nameLower) || strings.HasSuffix(nameLower, "."+symLower) {
			ext := strings.ToLower(filepath.Ext(sym.File))
			switch ext {
			case ".go":
				return "go"
			case ".odin":
				return "odin"
			case ".py":
				return "python"
			case ".kt":
				return "kotlin"
			case ".java":
				return "java"
			}
		}
	}
	return ""
}

func areLanguagesCompatible(lang1, lang2 string) bool {
	if lang1 == "" || lang2 == "" {
		return true
	}
	if lang1 == lang2 {
		return true
	}
	if (lang1 == "kotlin" && lang2 == "java") || (lang1 == "java" && lang2 == "kotlin") {
		return true
	}
	return false
}

// GetFile retrieves a registered file by its relative path, returning nil if not found.
func (s *Source) GetFile(name string) *File {
	for i := range s.Files {
		if s.Files[i].Name == name {
			return &s.Files[i]
		}
	}
	return nil
}

// AddFile registers a new file in the source database if it doesn't already exist.
func (s *Source) AddFile(name string) {
	if s.GetFile(name) == nil {
		s.Files = append(s.Files, File{Name: name})
	}
}

// AddSymbol registers a newly parsed symbol into the source database.
func (s *Source) AddSymbol(sym Symbol) {
	s.Symbols = append(s.Symbols, sym)
}

func sanitizeIdentifier(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", "_")
	s = strings.ReplaceAll(s, "\r", "_")
	s = strings.ReplaceAll(s, "\t", "_")

	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '.' || ch == '_' || ch == '-' {
			sb.WriteByte(ch)
		} else {
			if sb.Len() > 0 && sb.String()[sb.Len()-1] != '_' {
				sb.WriteByte('_')
			}
		}
	}
	res := sb.String()
	res = strings.Trim(res, "_")
	if len(res) > 80 {
		res = res[:80]
	}
	return res
}

// AddCall registers a newly identified call relation, ensuring duplicates are avoided.
func (s *Source) AddCall(caller, callee string) {
	caller = sanitizeIdentifier(caller)
	callee = sanitizeIdentifier(callee)
	if caller == "" || callee == "" {
		return
	}

	if s.callsSet == nil {
		s.callsSet = make(map[CallRelation]bool)
		for _, c := range s.Calls {
			s.callsSet[c] = true
		}
	}

	rel := CallRelation{Caller: caller, Callee: callee}
	if s.callsSet[rel] {
		return
	}

	s.Calls = append(s.Calls, rel)
	s.callsSet[rel] = true
	s.indexesBuilt = false
}

// GetCallers retrieves all direct callers for a given callee symbol.
func (s *Source) GetCallers(symbolName string) []string {
	if !s.indexesBuilt {
		s.buildIndexes()
	}
	res := s.callersIndex[strings.ToLower(symbolName)]
	if res == nil {
		return nil
	}
	out := make([]string, len(res))
	copy(out, res)
	return out
}

// GetCallees retrieves all direct callees for a given caller symbol.
func (s *Source) GetCallees(symbolName string) []string {
	if !s.indexesBuilt {
		s.buildIndexes()
	}
	res := s.calleesIndex[strings.ToLower(symbolName)]
	if res == nil {
		return nil
	}
	out := make([]string, len(res))
	copy(out, res)
	return out
}

// SearchSymbols returns symbols whose names contain the query (case-insensitive).
func (s *Source) SearchSymbols(query string) []Symbol {
	var result []Symbol
	q := strings.ToLower(query)
	for _, sym := range s.Symbols {
		if strings.Contains(strings.ToLower(sym.Name), q) {
			result = append(result, sym)
		}
	}
	return result
}

// FilterByAudience returns symbols that have the specified audience tag.
func (s *Source) FilterByAudience(aud string) []Symbol {
	var result []Symbol
	for _, sym := range s.Symbols {
		for _, tag := range sym.Audience {
			if strings.EqualFold(tag, aud) {
				result = append(result, sym)
				break
			}
		}
	}
	return result
}

// GetStructFields returns all fields for a given struct.
func (s *Source) GetStructFields(structName string) []Symbol {
	var result []Symbol
	for _, sym := range s.Symbols {
		if sym.Kind == SymField && sym.Parent == structName {
			result = append(result, sym)
		}
	}
	return result
}

// GetStructMethods returns all methods for a given struct.
func (s *Source) GetStructMethods(structName string) []Symbol {
	var result []Symbol
	for _, sym := range s.Symbols {
		if sym.Kind == SymMethod && sym.Parent == structName {
			result = append(result, sym)
		}
	}
	return result
}

// CoverBlock represents a statement block in a cover profile report.
type CoverBlock struct {
	File      string
	StartLine int
	EndLine   int
	NumStmt   int
	Count     int
}

// ParseCoverage parses any supported coverage profile (Go cover profile, LCOV, or CCOV).
func ParseCoverage(path string) ([]CoverBlock, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var blocks []CoverBlock
	scanner := bufio.NewScanner(file)

	// Read first few lines to detect format
	var lines []string
	isGoCover := false
	isLCOV := false
	isCCOV := false

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= 10 {
			break
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "mode:") {
			isGoCover = true
			break
		}
		if strings.HasPrefix(line, "SF:") || strings.HasPrefix(line, "DA:") {
			isLCOV = true
			break
		}
	}

	// If neither GoCover nor LCOV is detected, check if it's CCOV or check file extension
	if !isGoCover && !isLCOV {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".ccov" {
			isCCOV = true
		} else if ext == ".info" || ext == ".lcov" {
			isLCOV = true
		} else {
			// fallback check of content
			for _, line := range lines {
				if strings.Contains(line, ":") {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						if _, err := strconv.Atoi(parts[1]); err == nil {
							isCCOV = true
							break
						}
					}
				}
			}
		}
	}

	currentFile := ""
	parseLine := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}

		if isGoCover {
			if strings.HasPrefix(line, "mode:") {
				return
			}
			parts := strings.Fields(line)
			if len(parts) != 3 {
				return
			}
			filePart := parts[0]
			numStmt, _ := strconv.Atoi(parts[1])
			count, _ := strconv.Atoi(parts[2])

			colonIdx := strings.LastIndex(filePart, ":")
			if colonIdx == -1 {
				return
			}
			fileName := filePart[:colonIdx]
			rangePart := filePart[colonIdx+1:]

			commaIdx := strings.Index(rangePart, ",")
			if commaIdx == -1 {
				return
			}
			startPart := rangePart[:commaIdx]
			endPart := rangePart[commaIdx+1:]

			startDot := strings.Index(startPart, ".")
			if startDot != -1 {
				startPart = startPart[:startDot]
			}
			endDot := strings.Index(endPart, ".")
			if endDot != -1 {
				endPart = endPart[:endDot]
			}

			startLine, _ := strconv.Atoi(startPart)
			endLine, _ := strconv.Atoi(endPart)

			blocks = append(blocks, CoverBlock{
				File:      fileName,
				StartLine: startLine,
				EndLine:   endLine,
				NumStmt:   numStmt,
				Count:     count,
			})
		} else if isLCOV {
			if strings.HasPrefix(line, "SF:") {
				currentFile = strings.TrimPrefix(line, "SF:")
			} else if strings.HasPrefix(line, "DA:") {
				parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
				if len(parts) >= 2 && currentFile != "" {
					lineNum, _ := strconv.Atoi(parts[0])
					count, _ := strconv.Atoi(parts[1])
					blocks = append(blocks, CoverBlock{
						File:      currentFile,
						StartLine: lineNum,
						EndLine:   lineNum,
						NumStmt:   1,
						Count:     count,
					})
				}
			}
		} else if isCCOV {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				fileName := parts[0]
				lineNum, err := strconv.Atoi(parts[1])
				if err == nil {
					count := 1
					if len(parts) >= 3 {
						if c, err := strconv.Atoi(parts[2]); err == nil {
							count = c
						}
					}
					blocks = append(blocks, CoverBlock{
						File:      fileName,
						StartLine: lineNum,
						EndLine:   lineNum,
						NumStmt:   1,
						Count:     count,
					})
				}
			}
		}
	}

	for _, line := range lines {
		parseLine(line)
	}

	for scanner.Scan() {
		parseLine(scanner.Text())
	}

	return blocks, scanner.Err()
}

// ApplyCoverage calculates and attaches statement coverage percentages to functions and methods.
func (s *Source) ApplyCoverage(blocks []CoverBlock) {
	for i := range s.Symbols {
		sym := &s.Symbols[i]
		if sym.Kind != SymFunction && sym.Kind != SymMethod {
			continue
		}

		var totalStmts int
		var coveredStmts int
		for _, block := range blocks {
			// Match file (accept relative path match or exact match)
			if (block.File == sym.File || strings.HasSuffix(block.File, "/"+sym.File) || strings.HasSuffix(sym.File, "/"+block.File)) &&
				block.StartLine >= sym.Line && block.EndLine <= sym.Line+sym.LineCount {
				totalStmts += block.NumStmt
				if block.Count > 0 {
					coveredStmts += block.NumStmt
				}
			}
		}

		if totalStmts > 0 {
			cov := (float64(coveredStmts) / float64(totalStmts)) * 100.0
			sym.Coverage = &cov
		}
	}
}

