package frontend

import (
	"regexp"
	"strings"
	"doc_generator/pkg/store"
)

// Preprocess cleans TypeScript source code into standard Javascript suitable for tree-sitter-javascript.
// It also extracts detected 'interface' and 'type' aliases into the provided symbol store before deletion.
func Preprocess(source []byte, fileName string, symStore *store.Source) []byte {
	src := string(source)

	// Step 1: Extract interfaces
	extractTypesAndInterfaces(src, fileName, symStore)

	// Step 2: Strip 'type' and 'interface' declarations from the executable flow
	src = stripDeclarations(src)

	// Step 3: Strip generics like <T> from class signatures and function calls
	// Note: we replace with equivalent whitespace to keep line numbers relatively stable
	src = stripGenerics(src)

	// Step 4: Strip basic inline colon type annotations `: type` or `): type`
	src = stripTypeAnnotations(src)

	// Step 5: Strip 'as Type' casting
	src = stripCasts(src)

	// Step 6: Clean up remaining keywords that cause JS syntax errors (readonly, public, private)
	src = stripTSModifiers(src)

	return []byte(src)
}

var (
	reInterface = regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`)
	reType      = regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)\s*=`)
)

func extractTypesAndInterfaces(src, fileName string, symStore *store.Source) {
	// Fast finding of lines for basic location info
	lines := strings.Split(src, "\n")

	matches := reInterface.FindAllStringSubmatchIndex(src, -1)
	for _, m := range matches {
		if len(m) >= 4 {
			name := src[m[2]:m[3]]
			lineNum := getLineNumber(src, m[0], lines)
			symStore.AddSymbol(store.Symbol{
				Name: name,
				Kind: store.SymInterface,
				File: fileName,
				Line: lineNum,
			})
		}
	}

	matchesType := reType.FindAllStringSubmatchIndex(src, -1)
	for _, m := range matchesType {
		if len(m) >= 4 {
			name := src[m[2]:m[3]]
			lineNum := getLineNumber(src, m[0], lines)
			symStore.AddSymbol(store.Symbol{
				Name: name,
				Kind: store.SymInterface, // Treat type definitions as interfaces for simpler modeling
				File: fileName,
				Line: lineNum,
			})
		}
	}
}

func getLineNumber(fullSrc string, byteOffset int, lines []string) int {
	count := 0
	for i, line := range lines {
		count += len(line) + 1 // +1 for \n
		if count > byteOffset {
			return i + 1
		}
	}
	return 1
}

func stripDeclarations(src string) string {
	// Replaces 'interface' and 'type' blocks with spaces of equal size.
	// Simple brace matching approach
	output := []rune(src)
	
	keywords := []string{"interface ", "type "}
	for _, kw := range keywords {
		pos := 0
		for {
			idx := strings.Index(string(output[pos:]), kw)
			if idx == -1 {
				break
			}
			start := pos + idx
			
			// Basic heuristic to ensure it's at start of line or after export
			isActuallyDec := true
			if start > 0 {
				pre := strings.TrimSpace(string(output[max(0, start-10) : start]))
				if pre != "" && !strings.HasSuffix(pre, "export") && !strings.Contains(pre, "\n") && pre != " " {
					// Inside a comment or other context
					// For robust extraction, simplified logic applies here
				}
			}

			if isActuallyDec {
				// Find end of declaration
				// Scan forward for ';' or balanced braces
				end := findDeclarationEnd(output, start)
				if end > start {
					// White-out the region to maintain line offsets
					for k := start; k < end; k++ {
						if output[k] != '\n' {
							output[k] = ' '
						}
					}
				}
			}
			pos = start + len(kw)
		}
	}
	return string(output)
}

func findDeclarationEnd(runes []rune, start int) int {
	braceLevel := 0
	startedBraces := false
	for i := start; i < len(runes); i++ {
		r := runes[i]
		if r == '{' {
			braceLevel++
			startedBraces = true
		} else if r == '}' {
			braceLevel--
		} else if r == ';' && braceLevel == 0 {
			return i + 1
		}
		if startedBraces && braceLevel == 0 {
			return i + 1
		}
	}
	return len(runes)
}

var (
	reGen = regexp.MustCompile(`<[A-Z][\w\s,<>|&]*>`) // Simple capital letter-starting generic matches Component<Props>
)

func stripGenerics(src string) string {
	// Replaces generic clauses like `<Props>` or `<any>` with spaces.
	return reGen.ReplaceAllStringFunc(src, func(m string) string {
		return strings.Repeat(" ", len(m))
	})
}

var (
	// Match `: string`, `: Props`, `: (a:b)=>c` cases after parameter lists or names.
	// Order of alternation matters: more specific ones first.
	reColon = regexp.MustCompile(`:\s*(?:JSX\.Element|[A-Z][\w\.]*|string|number|boolean|void|any|unknown|never|object)(?:\s*\[\s*\])?`)
)

func stripTypeAnnotations(src string) string {
	return reColon.ReplaceAllStringFunc(src, func(m string) string {
		return strings.Repeat(" ", len(m))
	})
}

var (
	reCast = regexp.MustCompile(`\s+as\s+(?:[A-Za-z]\w*(?:\.Element)?|string|number)`)
)

func stripCasts(src string) string {
	return reCast.ReplaceAllStringFunc(src, func(m string) string {
		return strings.Repeat(" ", len(m))
	})
}

var (
	reMods = regexp.MustCompile(`\b(readonly|public|private|protected|implements)\b`)
)

func stripTSModifiers(src string) string {
	return reMods.ReplaceAllStringFunc(src, func(m string) string {
		// Implements can sometimes break inheritance trees in simple JS parsing.
		// Strip access modifiers which aren't valid JS property syntax without ESnext flags
		return strings.Repeat(" ", len(m))
	})
}

func max(a, b int) int {
	if a > b { return a }
	return b
}
