package main

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"doc_generator/pkg/store"
)

// Parser is the exported parser implementation.
var Parser store.Parser = &OdinParser{}

// Extensions is the list of file extensions this parser handles.
var Extensions = []string{".odin"}

// ---------------------------------------------------------------------------
// Regex patterns for Odin declarations — lazy-initialised to avoid conflicts
// with CGO-based plugin init ordering.
// ---------------------------------------------------------------------------

var (
	regexOnce sync.Once

	// package declaration: "package engine"
	rePackage *regexp.Regexp
	// import: `import "core:fmt"` or `import rl "vendor:raylib"` or `import "../engine"`
	reImport *regexp.Regexp
	// proc:   `name :: proc(` or with #force_inline etc.
	reProcDecl *regexp.Regexp
	// struct: `TypeName :: struct`
	reStructDecl *regexp.Regexp
	// enum:   `TypeName :: enum`
	reEnumDecl *regexp.Regexp
	// union:  `TypeName :: union`
	reUnionDecl *regexp.Regexp
	// bit_set: `TypeName :: bit_set[...]`
	reBitSetDecl *regexp.Regexp
	// interface: `TypeName :: interface`
	reInterfaceDecl *regexp.Regexp
	// distinct: `TypeName :: distinct BaseType`
	reDistinctDecl *regexp.Regexp
	// Type alias: `TypeName :: BaseType`
	reTypeAlias *regexp.Regexp
	// Constants: ALL_CAPS :: value
	reConstDecl *regexp.Regexp
	// Struct field: `field_name: Type`
	reStructField *regexp.Regexp
	// Enum variant line
	reEnumVariant *regexp.Regexp
	// @(private) or @(private = "file") attribute
	reAttrPrivate *regexp.Regexp
	// Call expressions inside proc body: identifier(
	reCallExpr *regexp.Regexp
	// Complexity tokens
	reComplexityTokens *regexp.Regexp
	// foreign block
	reForeignBlock *regexp.Regexp
	// Comment line
	reComment *regexp.Regexp
	// TODO in comment
	reTODO *regexp.Regexp
)

// initRegexps compiles all regular expressions used by the parser.
// Uses POSIX character classes instead of Perl \s / \w to avoid
// conflicts with CGO-loaded plugin environments.
func initRegexps() {
	sp := `[ \t\r\n\f\v]`  // whitespace
	w := `[A-Za-z0-9_]`    // word character
	W := `[^A-Za-z0-9_]`   // non-word character (for boundary)
	_ = W

	rePackage = regexp.MustCompile(`^` + sp + `*package` + sp + `+(` + w + `+)`)
	reImport = regexp.MustCompile(`^` + sp + `*import` + sp + `+(?:(` + w + `+)` + sp + `+)?"([^"]+)"`)
	reProcDecl = regexp.MustCompile(`^(` + w + `+)` + sp + `*::` + sp + `*(?:#` + w + `+` + sp + `+)*proc(?:` + sp + `|[\(\{\-]|$)`)
	reStructDecl = regexp.MustCompile(`^(` + w + `+)` + sp + `*::` + sp + `*struct(?:` + sp + `|[\{\-]|$)`)
	reEnumDecl = regexp.MustCompile(`^(` + w + `+)` + sp + `*::` + sp + `*enum(?:` + sp + `|[\{\-]|$)`)
	reUnionDecl = regexp.MustCompile(`^(` + w + `+)` + sp + `*::` + sp + `*union(?:` + sp + `|[\{\-]|$)`)
	reBitSetDecl = regexp.MustCompile(`^(` + w + `+)` + sp + `*::` + sp + `*bit_set(?:` + sp + `|[\[]|$)`)
	reInterfaceDecl = regexp.MustCompile(`^(` + w + `+)` + sp + `*::` + sp + `*interface(?:` + sp + `|[\(\{\-]|$)`)
	reDistinctDecl = regexp.MustCompile(`^(` + w + `+)` + sp + `*::` + sp + `*distinct` + sp + `+([^;\{\n]+)`)
	reTypeAlias = regexp.MustCompile(`^(` + w + `+)` + sp + `*::` + sp + `*(` + w + `+(?:\.` + w + `+)*)` + sp + `*$`)
	reConstDecl = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)` + sp + `*::` + sp + `*(.+)$`)
	// field: `name: type` or `n1, n2: type` — allow leading whitespace, colon separator
	reStructField = regexp.MustCompile(`^` + sp + `*(?:using` + sp + `+)?(` + w + `+(?:` + sp + `*,` + sp + `*` + w + `+)*)` + sp + `*:` + sp + `*([^,{}\n]+?)` + sp + `*,?` + sp + `*(?://.*)?$`)
	// enum variant: optional leading space, identifier, optional = value, optional comma, optional comment
	reEnumVariant = regexp.MustCompile(`^` + sp + `*(` + w + `+)(?:` + sp + `*=[^,]+)?` + sp + `*,?` + sp + `*(?://.*)?$`)
	reAttrPrivate = regexp.MustCompile(`@\(` + sp + `*private(?:` + sp + `*=` + sp + `*"[^"]*")?` + sp + `*\)`)
	// call: identifier (optional package/struct prefix) followed by opening paren
	reCallExpr = regexp.MustCompile(`\b((?:` + w + `+\.)?` + w + `+)` + sp + `*\(`)
	// complexity keywords
	reComplexityTokens = regexp.MustCompile(`\b(if|for|switch|when|case)\b`)
	reForeignBlock = regexp.MustCompile(`^` + sp + `*foreign` + sp + `+(` + w + `+)`)
	// comment
	reComment = regexp.MustCompile(`^` + sp + `*//`)
	// TODO
	reTODO = regexp.MustCompile(`TODO`)
}

// ensureRegexps lazily initialises all regexps on first use.
func ensureRegexps() {
	regexOnce.Do(initRegexps)
}

// ---------------------------------------------------------------------------
// OdinParser
// ---------------------------------------------------------------------------

// OdinParser implements store.Parser for .odin source files.
// It uses a line-by-line scanning approach with brace-counting to handle
// multi-line constructs, extracting procs, structs, enums, fields, constants,
// and imports with full doc-comment support and call-graph extraction.
type OdinParser struct {
	filePath string
	lines    []string
	source   *store.Source
	// Set of known struct/enum/union names for parent-proc heuristic.
	knownTypes map[string]bool
	// Package name parsed from the file.
	pkg string
}

// Parse satisfies store.Parser and is the main entry point for the plugin.
func (p *OdinParser) Parse(filePath string, fileContent []byte, source *store.Source) error {
	ensureRegexps()

	p.filePath = filePath
	p.source = source
	p.knownTypes = make(map[string]bool)
	p.lines = p.lines[:0]

	source.AddFile(filePath)

	// Split into lines (preserve originals for scanning).
	scanner := bufio.NewScanner(bytes.NewReader(fileContent))
	for scanner.Scan() {
		p.lines = append(p.lines, scanner.Text())
	}

	// Two-pass: first collect type names, then parse everything.
	p.collectTypeNames()
	p.parseAll()

	return nil
}

// collectTypeNames does a quick first pass to build the set of declared type
// names so the second pass can infer proc → struct associations.
func (p *OdinParser) collectTypeNames() {
	for _, line := range p.lines {
		trimmed := strings.TrimSpace(line)
		if m := reStructDecl.FindStringSubmatch(trimmed); m != nil {
			p.knownTypes[m[1]] = true
		}
		if m := reEnumDecl.FindStringSubmatch(trimmed); m != nil {
			p.knownTypes[m[1]] = true
		}
		if m := reUnionDecl.FindStringSubmatch(trimmed); m != nil {
			p.knownTypes[m[1]] = true
		}
		if m := reInterfaceDecl.FindStringSubmatch(trimmed); m != nil {
			p.knownTypes[m[1]] = true
		}
		if m := reDistinctDecl.FindStringSubmatch(trimmed); m != nil {
			p.knownTypes[m[1]] = true
		}
		if m := reTypeAlias.FindStringSubmatch(trimmed); m != nil {
			p.knownTypes[m[1]] = true
		}
	}
}

// parseAll iterates through lines and dispatches to specialised handlers.
func (p *OdinParser) parseAll() {
	i := 0
	pendingComments := []string{}
	pendingAttribs := []string{}

	for i < len(p.lines) {
		line := p.lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip empty lines — they reset the pending comment block.
		if trimmed == "" {
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			i++
			continue
		}

		// Package declaration.
		if m := rePackage.FindStringSubmatch(trimmed); m != nil {
			p.pkg = m[1]
			p.source.AddSymbol(store.Symbol{
				Name:    m[1],
				Kind:    "package",
				File:    p.filePath,
				Line:    i + 1,
				Package: m[1],
			})
			i++
			continue
		}

		// Import.
		if m := reImport.FindStringSubmatch(trimmed); m != nil {
			alias := m[1]
			path := m[2]
			name := path
			if alias != "" {
				name = alias
			} else {
				// use last part of path as name for graph purposes
				if idx := strings.LastIndex(path, ":"); idx != -1 {
					name = path[idx+1:]
				} else if idx := strings.LastIndex(path, "/"); idx != -1 {
					name = path[idx+1:]
				}
			}
			p.source.AddSymbol(store.Symbol{
				Name:    name,
				Kind:    store.SymImport,
				File:    p.filePath,
				Line:    i + 1,
				Type:    path, // store original path in Type field
				Package: p.pkg,
			})
			i++
			continue
		}

		// Attribute lines (@(private), etc.) — collect for next declaration.
		if strings.HasPrefix(trimmed, "@(") {
			pendingAttribs = append(pendingAttribs, trimmed)
			i++
			continue
		}

		// Feature pragmas like `#+feature dynamic-literals`.
		if strings.HasPrefix(trimmed, "#+") {
			i++
			continue
		}

		// Comment accumulation.
		if reComment.MatchString(trimmed) {
			comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
			pendingComments = append(pendingComments, comment)
			i++
			continue
		}

		// Struct declaration.
		if m := reStructDecl.FindStringSubmatch(trimmed); m != nil {
			doc := strings.Join(pendingComments, "\n")
			isPrivate := hasPrivateAttr(pendingAttribs)
			consumed := p.parseStruct(i, m[1], doc, isPrivate)
			i += consumed
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Enum declaration.
		if m := reEnumDecl.FindStringSubmatch(trimmed); m != nil {
			doc := strings.Join(pendingComments, "\n")
			isPrivate := hasPrivateAttr(pendingAttribs)
			consumed := p.parseEnum(i, m[1], doc, isPrivate)
			i += consumed
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Interface declaration.
		if m := reInterfaceDecl.FindStringSubmatch(trimmed); m != nil {
			doc := strings.Join(pendingComments, "\n")
			isPrivate := hasPrivateAttr(pendingAttribs)
			consumed := p.parseInterface(i, m[1], doc, isPrivate)
			i += consumed
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Union declaration.
		if m := reUnionDecl.FindStringSubmatch(trimmed); m != nil {
			doc := strings.Join(pendingComments, "\n")
			isPrivate := hasPrivateAttr(pendingAttribs)
			consumed := p.parseUnion(i, m[1], doc, isPrivate)
			i += consumed
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Bit-set declaration.
		if m := reBitSetDecl.FindStringSubmatch(trimmed); m != nil {
			doc := strings.Join(pendingComments, "\n")
			p.source.AddSymbol(store.Symbol{
				Name: m[1],
				Kind: store.SymStruct,
				File: p.filePath,
				Line: i + 1,
				Doc:  doc,
			})
			i++
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Distinct declaration.
		if m := reDistinctDecl.FindStringSubmatch(trimmed); m != nil {
			doc := strings.Join(pendingComments, "\n")
			baseType := strings.TrimSpace(m[2])
			p.source.AddSymbol(store.Symbol{
				Name:      m[1],
				Kind:      store.SymStruct,
				File:      p.filePath,
				Line:      i + 1,
				Doc:       doc,
				Type:      baseType,
				Package:   p.pkg,
				Relations: []string{baseType},
			})
			i++
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Type alias declaration.
		if m := reTypeAlias.FindStringSubmatch(trimmed); m != nil {
			doc := strings.Join(pendingComments, "\n")
			baseType := strings.TrimSpace(m[2])
			p.source.AddSymbol(store.Symbol{
				Name:      m[1],
				Kind:      store.SymStruct,
				File:      p.filePath,
				Line:      i + 1,
				Doc:       doc,
				Type:      baseType,
				Package:   p.pkg,
				Relations: []string{baseType},
			})
			i++
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Procedure declaration.
		if reProcDecl.MatchString(trimmed) {
			doc := strings.Join(pendingComments, "\n")
			isPrivate := hasPrivateAttr(pendingAttribs)
			consumed := p.parseProc(i, doc, isPrivate)
			i += consumed
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Foreign block.
		if m := reForeignBlock.FindStringSubmatch(trimmed); m != nil {
			consumed := p.parseForeign(i, m[1])
			i += consumed
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// TODO comments embedded inline.
		if reTODO.MatchString(trimmed) {
			p.source.AddSymbol(store.Symbol{
				Name: "TODO",
				Kind: store.SymVariable,
				File: p.filePath,
				Line: i + 1,
				Doc:  strings.TrimSpace(strings.TrimPrefix(trimmed, "//")),
			})
		}

		// All-caps constant (e.g., CONST_NAME :: value).
		if m := reConstDecl.FindStringSubmatch(trimmed); m != nil {
			if !p.knownTypes[m[1]] {
				doc := strings.Join(pendingComments, "\n")
				p.source.AddSymbol(store.Symbol{
					Name: m[1],
					Kind: store.SymVariable,
					File: p.filePath,
					Line: i + 1,
					Doc:  doc,
					Type: inferConstType(m[2]),
					Value: strings.TrimSpace(m[2]),
				})
			}
			i++
			pendingComments = pendingComments[:0]
			pendingAttribs = pendingAttribs[:0]
			continue
		}

		// Reset pending state for unrecognised lines.
		pendingComments = pendingComments[:0]
		pendingAttribs = pendingAttribs[:0]
		i++
	}
}

// ---------------------------------------------------------------------------
// Struct parsing
// ---------------------------------------------------------------------------

func (p *OdinParser) parseStruct(startLine int, name, doc string, isPrivate bool) int {
	_ = isPrivate
	p.source.AddSymbol(store.Symbol{
		Name:    name,
		Kind:    store.SymStruct,
		File:    p.filePath,
		Line:    startLine + 1,
		Doc:     doc,
		Package: p.pkg,
	})
	structIdx := len(p.source.Symbols) - 1

	depth := 0
	consumed := 0
	inBody := false

	for i := startLine; i < len(p.lines); i++ {
		consumed++
		line := p.lines[i]

		for _, ch := range line {
			if ch == '{' {
				depth++
				inBody = true
			} else if ch == '}' {
				depth--
			}
		}

		if inBody && depth == 1 {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "{" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			if m := reStructField.FindStringSubmatch(trimmed); m != nil {
				namesPart := m[1]
				fieldType := strings.TrimSpace(m[2])
				fieldType = strings.TrimRight(fieldType, ",")

				names := strings.Split(namesPart, ",")
				for _, n := range names {
					fName := strings.TrimSpace(n)
					if fName == "" {
						continue
					}
					p.source.AddSymbol(store.Symbol{
						Name:    fName,
						Kind:    store.SymField,
						File:    p.filePath,
						Line:    i + 1,
						Parent:  name,
						Type:    fieldType,
						Package: p.pkg,
					})
				}

				// Relationship extraction
				cleanType := stripTypeDecorators(fieldType)
				if p.knownTypes[cleanType] {
					found := false
					for _, r := range p.source.Symbols[structIdx].Relations {
						if r == cleanType {
							found = true
							break
						}
					}
					if !found {
						p.source.Symbols[structIdx].Relations = append(p.source.Symbols[structIdx].Relations, cleanType)
					}
				}
			}
		}

		if inBody && depth == 0 {
			break
		}
	}
	return consumed
}

// ---------------------------------------------------------------------------
// Enum parsing
// ---------------------------------------------------------------------------

func (p *OdinParser) parseEnum(startLine int, name, doc string, isPrivate bool) int {
	_ = isPrivate
	p.source.AddSymbol(store.Symbol{
		Name:    name,
		Kind:    store.SymInterface,
		File:    p.filePath,
		Line:    startLine + 1,
		Doc:     doc,
		Package: p.pkg,
	})

	depth := 0
	consumed := 0
	inBody := false

	for i := startLine; i < len(p.lines); i++ {
		consumed++
		line := p.lines[i]

		for _, ch := range line {
			if ch == '{' {
				depth++
				inBody = true
			} else if ch == '}' {
				depth--
			}
		}

		if inBody && depth == 1 {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "{" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			if idx := strings.Index(trimmed, "//"); idx != -1 {
				trimmed = strings.TrimSpace(trimmed[:idx])
			}
			if trimmed == "" {
				continue
			}
			if m := reEnumVariant.FindStringSubmatch(trimmed); m != nil {
				varName := m[1]
				if isValidIdentifier(varName) {
					p.source.AddSymbol(store.Symbol{
						Name:    varName,
						Kind:    store.SymField,
						File:    p.filePath,
						Line:    i + 1,
						Parent:  name,
						Package: p.pkg,
					})
				}
			}
		}

		if inBody && depth == 0 {
			break
		}
	}
	return consumed
}

// ---------------------------------------------------------------------------
// Interface parsing
// ---------------------------------------------------------------------------

func (p *OdinParser) parseInterface(startLine int, name, doc string, isPrivate bool) int {
	_ = isPrivate
	p.source.AddSymbol(store.Symbol{
		Name:    name,
		Kind:    store.SymInterface,
		File:    p.filePath,
		Line:    startLine + 1,
		Doc:     doc,
		Package: p.pkg,
	})

	depth := 0
	consumed := 0
	inBody := false

	for i := startLine; i < len(p.lines); i++ {
		consumed++
		line := p.lines[i]

		for _, ch := range line {
			if ch == '{' {
				depth++
				inBody = true
			} else if ch == '}' {
				depth--
			}
		}

		// Interfaces in Odin contain procedures or method requirements
		if inBody && depth >= 1 {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "{" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			// Look for procedure-like requirements
			if idx := strings.Index(trimmed, "("); idx != -1 {
				methodName := strings.TrimSpace(trimmed[:idx])
				if isValidIdentifier(methodName) {
					p.source.AddSymbol(store.Symbol{
						Name:    methodName,
						Kind:    store.SymMethod,
						File:    p.filePath,
						Line:    i + 1,
						Parent:  name,
						Package: p.pkg,
					})
				}
			}
		}

		if inBody && depth == 0 {
			break
		}
	}
	return consumed
}

// ---------------------------------------------------------------------------
// Union parsing
// ---------------------------------------------------------------------------

func (p *OdinParser) parseUnion(startLine int, name, doc string, isPrivate bool) int {
	_ = isPrivate
	p.source.AddSymbol(store.Symbol{
		Name:    name,
		Kind:    store.SymInterface,
		File:    p.filePath,
		Line:    startLine + 1,
		Doc:     doc,
		Package: p.pkg,
	})
	unionIdx := len(p.source.Symbols) - 1

	depth := 0
	consumed := 0
	inBody := false

	for i := startLine; i < len(p.lines); i++ {
		consumed++
		line := p.lines[i]

		for _, ch := range line {
			if ch == '{' {
				depth++
				inBody = true
			} else if ch == '}' {
				depth--
			}
		}

		if inBody && depth == 1 {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "{" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			// Union variants are often just types on separate lines.
			if idx := strings.Index(trimmed, "//"); idx != -1 {
				trimmed = strings.TrimSpace(trimmed[:idx])
			}
			trimmed = strings.TrimRight(trimmed, ",")
			if trimmed == "" {
				continue
			}

			// We treat union variants as fields of the union for documentation.
			cleanName := strings.TrimSpace(trimmed)
			if cleanName != "" {
				p.source.AddSymbol(store.Symbol{
					Name:    cleanName,
					Kind:    store.SymField,
					File:    p.filePath,
					Line:    i + 1,
					Parent:  name,
					Package: p.pkg,
				})

				// Relationship: Union variant "is a" member of the union
				variantType := stripTypeDecorators(cleanName)
				if p.knownTypes[variantType] {
					p.source.Symbols[unionIdx].Relations = append(p.source.Symbols[unionIdx].Relations, variantType)
				}
			}
		}

		if inBody && depth == 0 {
			break
		}
	}
	return consumed
}

// ---------------------------------------------------------------------------
// Procedure parsing
// ---------------------------------------------------------------------------

func (p *OdinParser) parseProc(startLine int, doc string, isPrivate bool) int {
	_ = isPrivate

	sigLines := []string{}
	bodyStart := -1
	consumed := 0

	for i := startLine; i < len(p.lines); i++ {
		consumed++
		line := p.lines[i]
		sigLines = append(sigLines, line)
		if strings.Contains(line, "{") || strings.HasSuffix(strings.TrimSpace(line), "---") {
			bodyStart = i
			break
		}
		if i > startLine && !strings.HasSuffix(strings.TrimSpace(line), ",") &&
			!strings.HasSuffix(strings.TrimSpace(line), "(") &&
			!strings.HasSuffix(strings.TrimSpace(line), "->") {
			break
		}
	}

	fullSig := strings.Join(sigLines, " ")
	name, params, returns := extractProcSignature(fullSig)
	if name == "" {
		return consumed
	}

	parent := p.inferParent(name, params)
	// If no struct parent, use package as parent for global functions to ensure unique qualified names.
	finalParent := parent
	if finalParent == "" {
		finalParent = p.pkg
	}

	lineCount := 1
	complexity := 1
	bodyLines := []string{}

	if bodyStart >= 0 && !strings.HasSuffix(strings.TrimSpace(p.lines[bodyStart]), "---") {
		depth := 0
		for i := bodyStart; i < len(p.lines); i++ {
			line := p.lines[i]
			for _, ch := range line {
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
				}
			}
			bodyLines = append(bodyLines, line)
			consumed = i - startLine + 1
			if depth == 0 {
				break
			}
			matches := reComplexityTokens.FindAllString(line, -1)
			complexity += len(matches)
		}
		lineCount = len(bodyLines)
	}

	callerName := name
	if parent != "" {
		callerName = parent + "." + name
	}
	if p.pkg != "" {
		callerName = p.pkg + "." + callerName
	}

	kind := store.SymFunction
	if parent != "" {
		kind = store.SymMethod
	}

	p.source.AddSymbol(store.Symbol{
		Name:       name,
		Kind:       kind,
		File:       p.filePath,
		Line:       startLine + 1,
		Doc:        doc,
		Parent:     parent, // Keep original struct parent for UI if any
		Package:    p.pkg,
		Params:     params,
		Returns:    returns,
		LineCount:  lineCount,
		Complexity: complexity,
	})
	// Record the qualified name in a internal-ish way or just use Parent.
	// Actually, if we want to be consistent with Go parser, we might want to NOT use package prefix for local calls.
	// But Odin has packages too.

	for _, bline := range bodyLines {
		matches := reCallExpr.FindAllStringSubmatch(bline, -1)
		for _, m := range matches {
			callee := m[1]
			if !isKeyword(callee) {
				p.source.AddCall(callerName, callee)
				// Also add a version without package prefix for local matching
				if idx := strings.Index(callee, "."); idx != -1 {
					p.source.AddCall(callerName, callee[idx+1:])
				}
			}
		}
	}

	return consumed
}

// ---------------------------------------------------------------------------
// Foreign block parsing
// ---------------------------------------------------------------------------

func (p *OdinParser) parseForeign(startLine int, libraryName string) int {
	depth := 0
	consumed := 0
	inBody := false
	pendingComments := []string{}

	for i := startLine; i < len(p.lines); {
		line := p.lines[i]
		trimmed := strings.TrimSpace(line)

		for _, ch := range line {
			if ch == '{' {
				depth++
				inBody = true
			} else if ch == '}' {
				depth--
			}
		}

		if inBody && depth >= 1 {
			if trimmed == "" || trimmed == "{" {
				i++
				consumed++
				continue
			}
			if reComment.MatchString(trimmed) {
				comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
				pendingComments = append(pendingComments, comment)
				i++
				consumed++
				continue
			}
			if reProcDecl.MatchString(trimmed) {
				doc := strings.Join(pendingComments, "\n")
				c := p.parseProc(i, doc, false)
				i += c
				consumed += c
				pendingComments = pendingComments[:0]
				continue
			}
		}

		i++
		consumed++
		if inBody && depth == 0 {
			break
		}
	}
	return consumed
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractProcSignature parses a joined multi-line procedure signature string.
func extractProcSignature(sig string) (name, params, returns string) {
	sp := `[ \t\r\n\f\v]`
	w := `[A-Za-z0-9_]`
	re := regexp.MustCompile(`(` + w + `+)` + sp + `*::` + sp + `*(?:#` + w + `+` + sp + `+)*proc(?:` + sp + `+"[^"]*")?` + sp + `*(\(.*?\))` + sp + `*(?:->` + sp + `*(.+?))?` + sp + `*(?:---|` + `\{` + `|$)`)
	m := re.FindStringSubmatch(sig)
	if m == nil {
		nameRe := regexp.MustCompile(`^(` + w + `+)` + sp + `*::`)
		nm := nameRe.FindStringSubmatch(strings.TrimSpace(sig))
		if nm != nil {
			name = nm[1]
		}
		return
	}
	name = m[1]
	params = m[2]
	returns = strings.TrimSpace(m[3])
	returns = strings.TrimRight(returns, " \t{")
	return
}

// inferParent checks if a proc relates to a type via first-parameter or snake_case convention.
func (p *OdinParser) inferParent(procName, params string) string {
	// 0. Extract package from procName if it's like "pkg.name"
	baseProcName := procName
	if idx := strings.Index(procName, "."); idx != -1 {
		baseProcName = procName[idx+1:]
	}

	// 1. Try first-parameter heuristic: (self: ^Type, ...)
	if params != "" {
		inner := strings.TrimSpace(params)
		if strings.HasPrefix(inner, "(") && strings.HasSuffix(inner, ")") {
			inner = inner[1 : len(inner)-1]
		}
		parts := strings.SplitN(inner, ",", 2)
		first := strings.TrimSpace(parts[0])
		if idx := strings.Index(first, ":"); idx != -1 {
			typ := strings.TrimSpace(first[idx+1:])
			if eqIdx := strings.Index(typ, "="); eqIdx != -1 {
				typ = strings.TrimSpace(typ[:eqIdx])
			}
			typ = stripTypeDecorators(typ)
			if p.knownTypes[typ] {
				return typ
			}
		}
	}

	// 2. Fallback to snake_case naming convention.
	for typeName := range p.knownTypes {
		snake := toSnakeCase(typeName)
		if snake != "" && strings.HasPrefix(baseProcName, snake+"_") {
			return typeName
		}
		lower := strings.ToLower(typeName)
		if lower != snake && strings.HasPrefix(baseProcName, lower+"_") {
			return typeName
		}
	}
	return ""
}

// stripTypeDecorators removes ^, [], [dynamic], etc. from a type string.
func stripTypeDecorators(typ string) string {
	typ = strings.TrimSpace(typ)
	for {
		old := typ
		if strings.HasPrefix(typ, "^") {
			typ = typ[1:]
		} else if strings.HasPrefix(typ, "[]") {
			typ = typ[2:]
		} else if strings.HasPrefix(typ, "[dynamic]") {
			typ = typ[9:]
		} else if strings.HasPrefix(typ, "map[") {
			// Find matching bracket for map[K]V
			depth := 0
			found := false
			for j, ch := range typ {
				if ch == '[' {
					depth++
				} else if ch == ']' {
					depth--
					if depth == 0 {
						typ = typ[j+1:]
						found = true
						break
					}
				}
			}
			if !found {
				break
			}
		} else if strings.HasPrefix(typ, "[") {
			// Handle [N]T
			if idx := strings.Index(typ, "]"); idx != -1 {
				typ = typ[idx+1:]
			} else {
				break
			}
		}

		typ = strings.TrimSpace(typ)
		if typ == old {
			break
		}
	}
	return typ
}

// toSnakeCase converts PascalCase to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (i+1 < len(runes) && unicode.IsLower(runes[i+1]) || unicode.IsLower(runes[i-1])) {
			result = append(result, '_')
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

// skipBlock scans forward until the matching closing brace.
func skipBlock(lines []string, startLine int) int {
	depth := 0
	for i := startLine; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
			}
		}
		if depth == 0 && i > startLine {
			return i - startLine + 1
		}
	}
	return len(lines) - startLine
}

// hasPrivateAttr returns true if any attribute marks private scope.
func hasPrivateAttr(attribs []string) bool {
	for _, a := range attribs {
		if reAttrPrivate.MatchString(a) {
			return true
		}
	}
	return false
}

// isValidIdentifier returns true for valid Odin identifiers.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if i > 0 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

// inferConstType guesses a human-readable type for a constant's value string.
func inferConstType(value string) string {
	v := strings.TrimSpace(value)
	if strings.HasPrefix(v, "\"") {
		return "string"
	}
	if strings.HasPrefix(v, "f32(") || strings.HasPrefix(v, "f64(") {
		return "float"
	}
	if strings.HasPrefix(v, "i32(") || strings.HasPrefix(v, "u8(") ||
		strings.HasPrefix(v, "u16(") || strings.HasPrefix(v, "u32(") {
		return "int"
	}
	if strings.HasPrefix(v, "true") || strings.HasPrefix(v, "false") {
		return "bool"
	}
	if strings.HasPrefix(v, "#config") {
		return "bool"
	}
	return ""
}

// isKeyword returns true for Odin language keywords.
func isKeyword(s string) bool {
	switch s {
	case "if", "else", "for", "in", "when", "switch", "case", "return",
		"proc", "struct", "enum", "union", "import", "package",
		"defer", "fallthrough", "break", "continue", "do",
		"make", "new", "delete", "append", "len", "cap",
		"transmute", "cast", "auto_cast", "context", "using",
		"map", "bit_set", "typeid", "any", "rawptr",
		"f32", "f64", "i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128", "uint", "int",
		"bool", "string", "cstring", "rune", "byte",
		"true", "false", "nil",
		"size_of", "align_of", "offset_of", "type_of", "type_info_of", "typeid_of",
		"swizzle", "complex", "real", "imag", "conj", "expand":
		return true
	}
	return false
}
