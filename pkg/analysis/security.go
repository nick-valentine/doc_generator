package analysis

import (
	"doc_generator/pkg/store"
	"os"
	"regexp"
	"strings"
)

// SecurityRule defines an individual regex heuristic to scan source files.
type SecurityRule struct {
	ID          string
	Severity    string // "Critical", "High", "Medium", "Low"
	Category    string // e.g., Injection, System Call
	Pattern     *regexp.Regexp
	Description string
}

// RunSecurityAnalysis sweeps function implementations across all target languages,
// flagging potential OWASP Top 10 vulnerabilities, OS command hooks, and outbound web queries.
func RunSecurityAnalysis(source *store.Source) {
	rules := []SecurityRule{
		// 1. OS System Command Execution
		{
			ID:          "SEC-CMD-EXEC",
			Severity:    "Critical",
			Category:    "System Call",
			Pattern:     regexp.MustCompile(`exec\.Command\(|os\.StartProcess\(|child_process\.exec\(|subprocess\.Popen\(|os\.system\(|Runtime\.getRuntime\(\)\.exec\(|os\.run_command\(|os\.execute\(`),
			Description: "System Command Invocation: detected execution of OS shell commands which could facilitate arbitrary code execution.",
		},
		// 2. SQL Injection Vulnerabilities
		{
			ID:          "SEC-SQLI",
			Severity:    "High",
			Category:    "Injection",
			Pattern:     regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE)\s+.*\s*\+\s*\w+|(?i)db\.Exec\(".*%s|(?i)db\.Query\(".*%s|(?i)SELECT\s.*\$\{`),
			Description: "Potential SQL Injection: detected string concatenation or unescaped format variables inside dynamic database query strings.",
		},
		// 3. Unsafe/Dynamic Eval
		{
			ID:          "SEC-EVAL",
			Severity:    "High",
			Category:    "Unsafe Execution",
			Pattern:     regexp.MustCompile(`(?i)eval\(|new\s+Function\(|unsafe\.Pointer`),
			Description: "Unsafe/Dynamic Execution: usage of runtime evaluators or pointer conversions bypassing memory safety.",
		},
		// 4. Hardcoded Access Credentials
		{
			ID:          "SEC-SECRET",
			Severity:    "High",
			Category:    "Hardcoded Secret",
			Pattern:     regexp.MustCompile(`(?i)(api_key|secret|password|token|access_token|private_key)\s*(:=|=|:)\s*(["'][A-Za-z0-9\-_\.]{16,}["'])`),
			Description: "Potential Hardcoded Secret: detected literal string token assignment directly attached to an authentication identifier.",
		},
		// 5. Path Traversal Hooks
		{
			ID:          "SEC-TRAVERSAL",
			Severity:    "Medium",
			Category:    "Path Traversal",
			Pattern:     regexp.MustCompile(`(?i)os\.Open\(\s*.*\+\s*|(?i)ioutil\.ReadFile\(\s*.*\+\s*|(?i)fs\.readFile\(\s*.*\+\s*|(?i)filepath\.Join\(\s*.*\+\s*`),
			Description: "Potential Path Traversal: raw file read path constructed via un-sanitized variable concatenation.",
		},
		// 6. Weak Cryptography Usage
		{
			ID:          "SEC-CRYPTO",
			Severity:    "Medium",
			Category:    "Weak Cryptography",
			Pattern:     regexp.MustCompile(`(?i)md5\.New\(|sha1\.New\(|crypto/md5|crypto/sha1|(?i)\.md5\(|(?i)\.sha1\(|DES_encrypt`),
			Description: "Outdated Hashing Primitives: usage of MD5/SHA1 hash implementations which are collision-vulnerable.",
		},
		// 7. Network Egress / Curl & Web Invocations
		{
			ID:          "SEC-NET-OUT",
			Severity:    "Low",
			Category:    "Network Access",
			Pattern:     regexp.MustCompile(`http\.Get\(|http\.Post\(|fetch\(|axios\.|requests\.get\(|requests\.post\(|urllib\.request|net\.Dial|net\.dial_tcp|net\.dial_udp|net\.DialTimeout`),
			Description: "External Network Action: discovered web/socket handles facilitating dynamic outbound TCP/HTTP requests.",
		},
	}

	// Memory-mapped file content cache to accelerate multi-symbol scans per source file
	fileCache := make(map[string][]string)

	for _, sym := range source.Symbols {
		// We restrict deep scanning to executable blocks to filter structural metadata false positives
		if sym.Kind != store.SymFunction && sym.Kind != store.SymMethod {
			continue
		}

		// Read lines from the cache or filesystem
		lines, exists := fileCache[sym.File]
		if !exists {
			data, err := os.ReadFile(sym.File)
			if err != nil {
				// Record skip to avoid repeated disk errors on broken links
				fileCache[sym.File] = nil
				continue
			}
			lines = strings.Split(string(data), "\n")
			fileCache[sym.File] = lines
		}

		if lines == nil {
			continue
		}

		// Isolate code implementation window
		startIdx := sym.Line - 1
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + sym.LineCount
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		if startIdx >= len(lines) {
			continue
		}

		// Combine context block for multiline regex detection
		snippetBlock := strings.Join(lines[startIdx:endIdx], "\n")

		// Execute regex rules
		for _, rule := range rules {
			matches := rule.Pattern.FindAllStringIndex(snippetBlock, -1)
			for _, match := range matches {
				// Trace relative newline offsets back to absolute source file lines
				beforeMatch := snippetBlock[:match[0]]
				matchedOffset := strings.Count(beforeMatch, "\n")
				absoluteLine := sym.Line + matchedOffset

				matchedText := ""
				blockLines := strings.Split(snippetBlock, "\n")
				if matchedOffset >= 0 && matchedOffset < len(blockLines) {
					matchedText = strings.TrimSpace(blockLines[matchedOffset])
				}

				symFullName := sym.Name
				if sym.Parent != "" {
					symFullName = sym.Parent + "." + sym.Name
				}

				// Deduplicate: ensure we don't log the exact same rule-and-line combo twice per symbol
				existsAlready := false
				for _, existing := range source.SecurityFindings {
					if existing.SymbolName == symFullName && existing.File == sym.File && existing.Line == absoluteLine && existing.Category == rule.Category {
						existsAlready = true
						break
					}
				}

				if !existsAlready {
					source.SecurityFindings = append(source.SecurityFindings, store.SecurityFinding{
						SymbolName:  symFullName,
						File:        sym.File,
						Line:        absoluteLine,
						Severity:    rule.Severity,
						Category:    rule.Category,
						Description: rule.Description,
						CodeSnippet: matchedText,
					})
				}
			}
		}
	}
}
