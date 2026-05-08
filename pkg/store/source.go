package store

import "strings"

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
	// Parent holds the name of the parent struct for fields and methods.
	Parent        string
	// Params holds the parameters of a function or method.
	Params        string
	// Returns holds the return type(s) of a function or method.
	Returns       string
	// Type holds the type of a field or variable.
	Type          string
	// LineCount is the number of lines of a function/method.
	LineCount     int
	// Complexity is the estimated cognitive/cyclomatic complexity.
	Complexity    int
	// Relations stores types that this symbol inherits from, implements, or is composed of.
	Relations     []string
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

// AddCall registers a newly identified call relation, ensuring duplicates are avoided.
func (s *Source) AddCall(caller, callee string) {
	caller = strings.TrimSpace(caller)
	callee = strings.TrimSpace(callee)
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
