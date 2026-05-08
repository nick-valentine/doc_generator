package generators

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"doc_generator/pkg/store"
)

// MarkdownGenerator is an output plugin that implements store.Generator.
// It formats the parsed files, structs, fields, and methods into an elegant Markdown document.
type MarkdownGenerator struct{}

// Generate builds a structured Markdown string summarizing all files and symbols, and writes it to outputDir.
func (mg *MarkdownGenerator) Generate(source *store.Source, outputDir string) error {
	var buf bytes.Buffer

	buf.WriteString("# Documentation Summary\n\n")

	// 1. Files Section
	buf.WriteString("## Files\n")
	if len(source.Files) == 0 {
		buf.WriteString("*No files parsed.*\n")
	} else {
		for _, f := range source.Files {
			buf.WriteString(fmt.Sprintf("- %s\n", f.Name))
		}
	}
	buf.WriteString("\n")

	// Documents Section
	buf.WriteString("## Documents\n\n")
	markdowns := getSymbolsOfKind(source, "markdown")
	if len(markdowns) == 0 {
		buf.WriteString("*No documents documented.*\n\n")
	} else {
		for _, md := range markdowns {
			buf.WriteString(fmt.Sprintf("### %s\n", md.Name))
			buf.WriteString(fmt.Sprintf("- **Location:** %s\n\n", md.File))
			buf.WriteString(md.Doc)
			buf.WriteString("\n\n---\n\n")
		}
	}

	// 2. Structs Section
	buf.WriteString("## Structures\n\n")
	structs := getSymbolsOfKind(source, store.SymStruct)
	if len(structs) == 0 {
		buf.WriteString("*No structs documented.*\n\n")
	} else {
		for _, strSym := range structs {
			buf.WriteString(fmt.Sprintf("### Struct: `%s`\n", strSym.Name))
			buf.WriteString(fmt.Sprintf("- **Location:** %s (Line %d)\n", strSym.File, strSym.Line))
			if len(strSym.Audience) > 0 {
				buf.WriteString(fmt.Sprintf("- **Audience:** %s\n", strings.Join(strSym.Audience, ", ")))
			}
			if len(strSym.Compatibility) > 0 {
				buf.WriteString(fmt.Sprintf("- **Compatibility:** %s\n", strings.Join(strSym.Compatibility, ", ")))
			}
			if strSym.Doc != "" {
				buf.WriteString(fmt.Sprintf("\n> %s\n", strings.ReplaceAll(strSym.Doc, "\n", "\n> ")))
			}

			// Struct Fields
			fields := source.GetStructFields(strSym.Name)
			if len(fields) > 0 {
				buf.WriteString("\n#### Fields\n")
				for _, field := range fields {
					buf.WriteString(fmt.Sprintf("- `%s`", field.Name))
					var meta []string
					if len(field.Audience) > 0 {
						meta = append(meta, fmt.Sprintf("Audience: %s", strings.Join(field.Audience, ", ")))
					}
					if len(field.Compatibility) > 0 {
						meta = append(meta, fmt.Sprintf("Compatibility: %s", strings.Join(field.Compatibility, ", ")))
					}
					if len(meta) > 0 {
						buf.WriteString(fmt.Sprintf(" *(%s)*", strings.Join(meta, " | ")))
					}
					buf.WriteString("\n")
					if field.Doc != "" {
						buf.WriteString(fmt.Sprintf("  > %s\n", strings.ReplaceAll(field.Doc, "\n", "\n  > ")))
					}
				}
			}

			// Struct Methods
			methods := source.GetStructMethods(strSym.Name)
			if len(methods) > 0 {
				buf.WriteString("\n#### Methods\n")
				for _, method := range methods {
					buf.WriteString(fmt.Sprintf("- `%s()`", method.Name))
					var meta []string
					if len(method.Audience) > 0 {
						meta = append(meta, fmt.Sprintf("Audience: %s", strings.Join(method.Audience, ", ")))
					}
					if len(method.Compatibility) > 0 {
						meta = append(meta, fmt.Sprintf("Compatibility: %s", strings.Join(method.Compatibility, ", ")))
					}
					if len(meta) > 0 {
						buf.WriteString(fmt.Sprintf(" *(%s)*", strings.Join(meta, " | ")))
					}
					buf.WriteString("\n")
					if method.Doc != "" {
						buf.WriteString(fmt.Sprintf("  > %s\n", strings.ReplaceAll(method.Doc, "\n", "\n  > ")))
					}

					// Caller/Callee Relations & Graph References
					methodKey := fmt.Sprintf("%s.%s", strSym.Name, method.Name)
					callers := source.GetCallers(methodKey)
					callees := source.GetCallees(methodKey)
					cleanKey := strings.ReplaceAll(methodKey, ".", "_")

					if len(callers) > 0 {
						buf.WriteString(fmt.Sprintf("  > **Callers:** %s\n", strings.Join(callers, ", ")))
					}
					if len(callees) > 0 {
						buf.WriteString(fmt.Sprintf("  > **Callees:** %s\n", strings.Join(callees, ", ")))
					}
					if len(callers) > 0 || len(callees) > 0 {
						buf.WriteString(fmt.Sprintf("  >\n  > **Call Graph:**\n  > ![Call Graph for %s](images/%s_call_graph.png)\n", methodKey, cleanKey))
					}
				}
			}
			buf.WriteString("\n---\n\n")
		}
	}

	// 3. Functions Section
	buf.WriteString("## Global Functions\n\n")
	funcs := getSymbolsOfKind(source, store.SymFunction)
	if len(funcs) == 0 {
		buf.WriteString("*No global functions documented.*\n\n")
	} else {
		for _, fnSym := range funcs {
			buf.WriteString(fmt.Sprintf("### Function: `%s`\n", fnSym.Name))
			buf.WriteString(fmt.Sprintf("- **Location:** %s (Line %d)\n", fnSym.File, fnSym.Line))
			if len(fnSym.Audience) > 0 {
				buf.WriteString(fmt.Sprintf("- **Audience:** %s\n", strings.Join(fnSym.Audience, ", ")))
			}
			if len(fnSym.Compatibility) > 0 {
				buf.WriteString(fmt.Sprintf("- **Compatibility:** %s\n", strings.Join(fnSym.Compatibility, ", ")))
			}
			if fnSym.Doc != "" {
				buf.WriteString(fmt.Sprintf("\n> %s\n", strings.ReplaceAll(fnSym.Doc, "\n", "\n> ")))
			}

			// Caller/Callee Relations & Graph References
			fnKey := fnSym.Name
			callers := source.GetCallers(fnKey)
			callees := source.GetCallees(fnKey)
			cleanKey := strings.ReplaceAll(fnKey, ".", "_")

			if len(callers) > 0 || len(callees) > 0 {
				buf.WriteString("\n")
			}
			if len(callers) > 0 {
				buf.WriteString(fmt.Sprintf("> **Callers:** %s\n", strings.Join(callers, ", ")))
			}
			if len(callees) > 0 {
				buf.WriteString(fmt.Sprintf("> **Callees:** %s\n", strings.Join(callees, ", ")))
			}
			if len(callers) > 0 || len(callees) > 0 {
				buf.WriteString(fmt.Sprintf(">\n> **Call Graph:**\n> ![Call Graph for %s](images/%s_call_graph.png)\n", fnKey, cleanKey))
			}

			buf.WriteString("\n---\n\n")
		}
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "index.md"), buf.Bytes(), 0644)
}

// getSymbolsOfKind is a private helper that filters all parsed symbols in the source store by their symbol type.
func getSymbolsOfKind(source *store.Source, kind store.SymbolType) []store.Symbol {
	var result []store.Symbol
	for _, sym := range source.Symbols {
		if sym.Kind == kind {
			result = append(result, sym)
		}
	}
	return result
}
