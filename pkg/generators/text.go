package generators

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"doc_generator/pkg/store"
)

// MarkdownGenerator is an output plugin that implements store.Generator.
// It formats the parsed files, structs, fields, and methods into an elegant Markdown document.
type MarkdownGenerator struct {
	Language string
}

// T translates the key using the generator's Language setting.
func (mg *MarkdownGenerator) T(key string) string {
	return Translate(mg.Language, key)
}

// Generate builds a structured Markdown string summarizing all files and symbols, and writes it to outputDir.
func (mg *MarkdownGenerator) Generate(source *store.Source, outputDir string) error {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("# %s\n\n", mg.T("doc_summary")))

	// 1. Files Section
	buf.WriteString(fmt.Sprintf("## %s\n", mg.T("files")))
	if len(source.Files) == 0 {
		buf.WriteString(fmt.Sprintf("*%s*\n", mg.T("no_files")))
	} else {
		for _, f := range source.Files {
			buf.WriteString(fmt.Sprintf("- %s\n", f.Name))
		}
	}
	buf.WriteString("\n")

	// Documents Section
	buf.WriteString(fmt.Sprintf("## %s\n\n", mg.T("documents")))
	markdowns := getSymbolsOfKind(source, "markdown")
	if len(markdowns) == 0 {
		buf.WriteString(fmt.Sprintf("*%s*\n\n", mg.T("no_documents")))
	} else {
		for _, md := range markdowns {
			buf.WriteString(fmt.Sprintf("### %s\n", md.Name))
			buf.WriteString(fmt.Sprintf("- **%s:** %s\n\n", mg.T("location"), md.File))
			buf.WriteString(md.Doc)
			buf.WriteString("\n\n---\n\n")
		}
	}

	// 2. Structs Section
	buf.WriteString(fmt.Sprintf("## %s\n\n", mg.T("structures")))
	structs := getSymbolsOfKind(source, store.SymStruct)
	if len(structs) == 0 {
		buf.WriteString(fmt.Sprintf("*%s*\n\n", mg.T("no_structs")))
	} else {
		for _, strSym := range structs {
			buf.WriteString(fmt.Sprintf("### %s: `%s`\n", mg.T("struct"), strSym.Name))
			buf.WriteString(fmt.Sprintf("- **%s:** %s (Line %d)\n", mg.T("location"), strSym.File, strSym.Line))
			if len(strSym.Relations) > 0 {
				buf.WriteString(fmt.Sprintf("- **%s:** %s\n", mg.T("relations"), strings.Join(strSym.Relations, ", ")))
			}
			if len(strSym.Audience) > 0 {
				buf.WriteString(fmt.Sprintf("- **%s:** %s\n", mg.T("audience"), strings.Join(strSym.Audience, ", ")))
			}
			if len(strSym.Compatibility) > 0 {
				buf.WriteString(fmt.Sprintf("- **%s:** %s\n", mg.T("compatibility"), strings.Join(strSym.Compatibility, ", ")))
			}
			if strSym.Doc != "" {
				buf.WriteString(fmt.Sprintf("\n> %s\n", strings.ReplaceAll(strSym.Doc, "\n", "\n> ")))
			}

			// Struct Fields
			fields := source.GetStructFields(strSym.Name)
			if len(fields) > 0 {
				buf.WriteString(fmt.Sprintf("\n#### %s\n", mg.T("fields")))
				for _, field := range fields {
					buf.WriteString(fmt.Sprintf("- `%s`", field.Name))
					var meta []string
					if len(field.Audience) > 0 {
						meta = append(meta, fmt.Sprintf("%s: %s", mg.T("audience"), strings.Join(field.Audience, ", ")))
					}
					if len(field.Compatibility) > 0 {
						meta = append(meta, fmt.Sprintf("%s: %s", mg.T("compatibility"), strings.Join(field.Compatibility, ", ")))
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
				buf.WriteString(fmt.Sprintf("\n#### %s\n", mg.T("methods")))
				for _, method := range methods {
					asyncTag := ""
					if method.IsAsync {
						asyncTag = fmt.Sprintf(" (%s)", mg.T("async"))
					}
					buf.WriteString(fmt.Sprintf("- `%s()`%s", method.Name, asyncTag))
					var meta []string
					if len(method.Audience) > 0 {
						meta = append(meta, fmt.Sprintf("%s: %s", mg.T("audience"), strings.Join(method.Audience, ", ")))
					}
					if len(method.Compatibility) > 0 {
						meta = append(meta, fmt.Sprintf("%s: %s", mg.T("compatibility"), strings.Join(method.Compatibility, ", ")))
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
						buf.WriteString(fmt.Sprintf("  > **%s:** %s\n", mg.T("callers"), strings.Join(callers, ", ")))
					}
					if len(callees) > 0 {
						buf.WriteString(fmt.Sprintf("  > **%s:** %s\n", mg.T("callees"), strings.Join(callees, ", ")))
					}
					if len(callers) > 0 || len(callees) > 0 {
						buf.WriteString(fmt.Sprintf("  >\n  > **%s:**\n  > ![%s %s](images/%s_call_graph.svg)\n", mg.T("call_graph"), mg.T("call_graph"), methodKey, cleanKey))
					}
				}
			}
			buf.WriteString("\n---\n\n")
		}
	}

	// 3. Functions Section
	buf.WriteString(fmt.Sprintf("## %s\n\n", mg.T("global_funcs")))
	funcs := getSymbolsOfKind(source, store.SymFunction)
	if len(funcs) == 0 {
		buf.WriteString(fmt.Sprintf("*%s*\n\n", mg.T("no_funcs")))
	} else {
		for _, fnSym := range funcs {
			asyncTag := ""
			if fnSym.IsAsync {
				asyncTag = fmt.Sprintf(" (%s)", mg.T("async"))
			}
			buf.WriteString(fmt.Sprintf("### %s: `%s` %s\n", mg.T("function"), fnSym.Name, asyncTag))
			buf.WriteString(fmt.Sprintf("- **%s:** %s (Line %d)\n", mg.T("location"), fnSym.File, fnSym.Line))
			if len(fnSym.Audience) > 0 {
				buf.WriteString(fmt.Sprintf("- **%s:** %s\n", mg.T("audience"), strings.Join(fnSym.Audience, ", ")))
			}
			if len(fnSym.Compatibility) > 0 {
				buf.WriteString(fmt.Sprintf("- **%s:** %s\n", mg.T("compatibility"), strings.Join(fnSym.Compatibility, ", ")))
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
				buf.WriteString(fmt.Sprintf("> **%s:** %s\n", mg.T("callers"), strings.Join(callers, ", ")))
			}
			if len(callees) > 0 {
				buf.WriteString(fmt.Sprintf("> **%s:** %s\n", mg.T("callees"), strings.Join(callees, ", ")))
			}
			if len(callers) > 0 || len(callees) > 0 {
				buf.WriteString(fmt.Sprintf(">\n> **%s:**\n> ![%s %s](images/%s_call_graph.svg)\n", mg.T("call_graph"), mg.T("call_graph"), fnKey, cleanKey))
			}

			buf.WriteString("\n---\n\n")
		}
	}

	// 4. Design Patterns Section
	buf.WriteString(fmt.Sprintf("## %s\n\n", mg.T("architecture_patterns")))
	if len(source.Patterns) == 0 {
		buf.WriteString(fmt.Sprintf("*%s*\n\n", mg.T("no_patterns")))
	} else {
		buf.WriteString(fmt.Sprintf("| %s | %s | %s |\n", mg.T("pattern_name"), "Category", mg.T("participating_symbols")))
		buf.WriteString("| --- | --- | --- |\n")
		for _, pat := range source.Patterns {
			buf.WriteString(fmt.Sprintf("| **%s** | %s | %s |\n", pat.Name, pat.Category, strings.Join(pat.Symbols, ", ")))
		}
		buf.WriteString("\n")
	}

	// 5. Network Analysis Section
	buf.WriteString(fmt.Sprintf("## %s\n\n", mg.T("network_analysis")))
	if len(source.NetworkAnalysis) == 0 {
		buf.WriteString(fmt.Sprintf("*%s*\n\n", mg.T("no_network")))
	} else {
		buf.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", mg.T("name"), mg.T("type"), mg.T("details"), mg.T("severity")))
		buf.WriteString("| --- | --- | --- | --- |\n")
		for _, netComp := range source.NetworkAnalysis {
			detailsText := ""
			var detailsKeys []string
			for k := range netComp.Details {
				detailsKeys = append(detailsKeys, k)
			}
			sort.Strings(detailsKeys)
			for _, k := range detailsKeys {
				v := netComp.Details[k]
				if k != "Security & Risk Context" {
					detailsText += fmt.Sprintf("**%s**: %s<br>", k, v)
				}
			}
			riskCtx := netComp.Details["Security & Risk Context"]
			if riskCtx == "" {
				riskCtx = "🟢 Low"
			}
			buf.WriteString(fmt.Sprintf("| **%s** | %s | %s | %s |\n", netComp.Name, netComp.Type, detailsText, riskCtx))
		}
		buf.WriteString("\n")
	}

	// 6. System & AI Security Audit
	buf.WriteString(fmt.Sprintf("## %s\n\n", mg.T("security_analysis")))
	if len(source.SecurityFindings) == 0 {
		buf.WriteString(fmt.Sprintf("*%s*\n\n", mg.T("clean_bill_desc")))
	} else {
		// Group counts
		sevCounts := make(map[string]int)
		for _, f := range source.SecurityFindings {
			sevCounts[f.Severity]++
		}

		buf.WriteString(fmt.Sprintf("- 🔴 **Critical**: %d\n", sevCounts["Critical"]))
		buf.WriteString(fmt.Sprintf("- 🟠 **High**: %d\n", sevCounts["High"]))
		buf.WriteString(fmt.Sprintf("- 🟡 **Medium**: %d\n", sevCounts["Medium"]))
		buf.WriteString(fmt.Sprintf("- 🔵 **Low**: %d\n\n", sevCounts["Low"]))

		buf.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", mg.T("th_severity"), "Location / Symbol", mg.T("th_category"), "Evidence"))
		buf.WriteString("| --- | --- | --- | --- |\n")

		// Sort by severity
		var orderedFindings []store.SecurityFinding
		for _, s := range []string{"Critical", "High", "Medium", "Low"} {
			for _, f := range source.SecurityFindings {
				if strings.EqualFold(f.Severity, s) {
					orderedFindings = append(orderedFindings, f)
				}
			}
		}

		for _, f := range orderedFindings {
			sevIcon := "🔵"
			switch strings.ToUpper(f.Severity) {
			case "CRITICAL":
				sevIcon = "🔴"
			case "HIGH":
				sevIcon = "🟠"
			case "MEDIUM":
				sevIcon = "🟡"
			}
			buf.WriteString(fmt.Sprintf("| %s **%s** | %s:%d (`%s`) | %s | %s |\n", sevIcon, f.Severity, filepath.Base(f.File), f.Line, f.SymbolName, f.Category, f.Description))
		}
		buf.WriteString("\n")
	}

	// 7. Unified Translations Matrix
	buf.WriteString(fmt.Sprintf("## %s\n\n", mg.T("translation_matrix")))
	if len(source.Translations) == 0 {
		buf.WriteString("*No localized translation assets resolved.*\n\n")
	} else {
		localesSet := make(map[string]bool)
		keysSet := make(map[string]bool)
		matrix := make(map[string]map[string]string)

		for _, tr := range source.Translations {
			localesSet[tr.Locale] = true
			keysSet[tr.Key] = true
			if matrix[tr.Key] == nil {
				matrix[tr.Key] = make(map[string]string)
			}
			matrix[tr.Key][tr.Locale] = tr.Value
		}

		var sortedLocales []string
		for loc := range localesSet {
			sortedLocales = append(sortedLocales, loc)
		}
		sort.Strings(sortedLocales)

		var sortedKeys []string
		for k := range keysSet {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)

		baseLocale := ""
		for _, l := range sortedLocales {
			lowerL := strings.ToLower(l)
			if lowerL == "en" || strings.HasPrefix(lowerL, "en_") || strings.HasPrefix(lowerL, "en-") {
				baseLocale = l
				break
			}
		}
		if baseLocale == "" && len(sortedLocales) > 0 {
			baseLocale = sortedLocales[0]
		}

		totalExpected := len(sortedKeys) * len(sortedLocales)
		completionPct := 0.0
		if totalExpected > 0 {
			completionPct = (float64(len(source.Translations)) / float64(totalExpected)) * 100.0
		}

		buf.WriteString(fmt.Sprintf("- **%s**: %d\n", mg.T("unique_keys"), len(sortedKeys)))
		buf.WriteString(fmt.Sprintf("- **%s**: %d\n", mg.T("locales"), len(sortedLocales)))
		buf.WriteString(fmt.Sprintf("- **%s**: %.1f%%\n\n", mg.T("global_coverage"), completionPct))

		// Language header
		buf.WriteString(fmt.Sprintf("| %s", mg.T("th_loc_key")))
		for _, loc := range sortedLocales {
			if loc == baseLocale {
				buf.WriteString(fmt.Sprintf(" | **%s** `%s`", loc, mg.T("th_base_loc")))
			} else {
				buf.WriteString(fmt.Sprintf(" | **%s**", loc))
			}
		}
		buf.WriteString(" |\n")

		// Separator
		buf.WriteString("| ---")
		for range sortedLocales {
			buf.WriteString(" | ---")
		}
		buf.WriteString(" |\n")

		// Key rows
		for _, k := range sortedKeys {
			buf.WriteString(fmt.Sprintf("| `%s`", k))
			baseVal := matrix[k][baseLocale]
			for _, loc := range sortedLocales {
				val := matrix[k][loc]
				if val == "" {
					buf.WriteString(fmt.Sprintf(" | *%s*", mg.T("missing_val")))
				} else if loc != baseLocale && val == baseVal && val != "" {
					buf.WriteString(fmt.Sprintf(" | ⚠️ *%s* (%s)", val, mg.T("copy_pasted_warn")))
				} else {
					buf.WriteString(fmt.Sprintf(" | %s", val))
				}
			}
			buf.WriteString(" |\n")
		}
		buf.WriteString("\n")
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
