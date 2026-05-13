package analysis

import (
	"doc_generator/pkg/store"
	"os"
	"regexp"
	"strings"
)

// SANRule defines a specific heuristic to scan both markdown and executable symbols.
type SANRule struct {
	ID          string
	Severity    string // "Critical", "High", "Medium", "Low"
	Category    string // "Prompt Injection", "AI Weakness", "Skill Vulnerability"
	Pattern     *regexp.Regexp
	Description string
	Scope       string // "all", "markdown", "code"
}

// RunSANAnalysis performs Systematic/Security & Adversarial Node (SAN) scanning
// across both source code functions and parsed markdown skills/docs.
func RunSANAnalysis(source *store.Source) {
	rules := []SANRule{
		// --- 1. Vetted Prompt Injections (Extracted from trusted pre-tool hooks) ---
		{
			ID:          "SAN-INJ-OVERRIDE",
			Severity:    "Critical",
			Category:    "Prompt Injection",
			Pattern:     regexp.MustCompile(`(?i)ignore\s+(all\s+)?(?:previous|above)\s+instructions|forget\s+(all\s+)?(?:your\s+)?instructions|disregard\s+(all\s+)?previous|override\s+(system|previous)\s+(prompt|instructions)`),
			Description: "Direct System Prompt Override: adversarial directive attempting to negate parent boundaries.",
			Scope:       "all",
		},
		{
			ID:          "SAN-INJ-IDENTITY",
			Severity:    "High",
			Category:    "Prompt Injection",
			Pattern:     regexp.MustCompile(`(?i)you\s+are\s+now\s+(?:a|an|the)\s+|pretend\s+(?:you(?:'re| are)\s+|to\s+be\s+)|from\s+now\s+on,?\s+you\s+(?:are|will|should|must)`),
			Description: "Role Assumption Exploit: attempts to coerce the agent node into adopting a non-sanctioned identity or instruction set.",
			Scope:       "all",
		},
		{
			ID:          "SAN-INJ-REVEAL",
			Severity:    "High",
			Category:    "Prompt Injection",
			Pattern:     regexp.MustCompile(`(?i)(?:print|output|reveal|show|display|repeat)\s+(?:your\s+)?(?:system\s+)?(?:prompt|instructions)`),
			Description: "Instruction Leakage Sweep: attempt to force serialization and exfiltration of hidden system instructions.",
			Scope:       "all",
		},
		{
			ID:          "SAN-INJ-TAGS",
			Severity:    "High",
			Category:    "Prompt Injection",
			Pattern:     regexp.MustCompile(`(?i)<\/?(?:system|assistant|human)>|\[SYSTEM\]|\[INST\]|<<\s*SYS\s*>>`),
			Description: "Boundary Token Smuggling: raw system prompt or instruction tag strings mimicking underlying model demarcation boundaries.",
			Scope:       "all",
		},
		{
			ID:          "SAN-INJ-CHATML",
			Severity:    "Critical",
			Category:    "Prompt Injection",
			Pattern:     regexp.MustCompile(`(?i)<\|im_start\|>|<\|im_end\|>|<\|system\|>|<\|user\|>|<\|assistant\|>`),
			Description: "ChatML Delimiter Confusion: specialized system-level delimiters designed to hijack prompt formatting layers.",
			Scope:       "all",
		},
		{
			ID:          "SAN-INJ-UNICODE",
			Severity:    "Medium",
			Category:    "Prompt Injection",
			Pattern:     regexp.MustCompile(`[\x{200b}-\x{200f}\x{2028}-\x{202f}\x{feff}\x{00ad}]`),
			Description: "Adversarial Unicode Noise: presence of hidden zero-width or directional characters often utilized for visual prompt obfuscation.",
			Scope:       "all",
		},
		{
			ID:          "SAN-INJ-OBFUSCATION",
			Severity:    "High",
			Category:    "Prompt Injection",
			Pattern:     regexp.MustCompile(`(?i)(?:decode\s+this|rot13|caesar\s+cipher|base64\s+decode|reverse\s+this\s+string|cipher\s+text)`),
			Description: "Obfuscated Payload Directive: adversarial instructions requiring the model to decrypt or decode split command fragments to evade outer scanners.",
			Scope:       "all",
		},

		// --- 2. Untrusted Code Implementation Vulnerabilities ---
		{
			ID:          "SAN-CODE-INTERP",
			Severity:    "High",
			Category:    "AI Weakness",
			Pattern:     regexp.MustCompile(`(?i)(?:prompt|query|instruction|template)\s*(?::=|=|\+)\s*.*(?:fmt\.Sprintf|strings\.Join|\+\s*\w+|Template.*Execute)`),
			Description: "Unsanitized Prompt Concatenation: dynamic string interpolation detected immediately assigned to an LLM prompt variable.",
			Scope:       "code",
		},
		{
			ID:          "SAN-CODE-EXFIL",
			Severity:    "High",
			Category:    "AI Weakness",
			Pattern:     regexp.MustCompile(`(?i)(?:curl|wget|fetch|http\.(?:Get|Post))\b[\s(]*.*(?:webhook|pastebin|leak|exfil|http://\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`),
			Description: "Potential Outbound Exfiltration Channel: functions making network requests containing exfiltration keywords or raw local IPs.",
			Scope:       "code",
		},

		// --- 3. Suspicious Skill Instruction Behavior (Phase 1/2 Skill Audit) ---
		{
			ID:          "SAN-SKILL-RCE",
			Severity:    "Critical",
			Category:    "Skill Vulnerability",
			Pattern:     regexp.MustCompile(`(?i)(?:curl|wget)\s+.*\s*\|\s*(?:bash|sh|zsh|python)`),
			Description: "Untrusted Execution Pipe: skill contains command configurations that download remote URLs and pipe them directly to shell nodes.",
			Scope:       "markdown",
		},
		{
			ID:          "SAN-SKILL-DROPPER",
			Severity:    "High",
			Category:    "Skill Vulnerability",
			Pattern:     regexp.MustCompile(`(?i)(?:atob|base64\s+-d|base64\s+--decode)\s*\(`),
			Description: "Encoded Asset Decoder: detected attempts to runtime decode obfuscated binary string payloads.",
			Scope:       "markdown",
		},
		{
			ID:          "SAN-SKILL-ENV",
			Severity:    "Medium",
			Category:    "Skill Vulnerability",
			Pattern:     regexp.MustCompile(`(?i)(?:\.env|process\.env|os\.Getenv|~\/\.bashrc|~\/\.ssh)`),
			Description: "Credential Targeting: markdown instructions demanding access to sensitive runtime configuration or key store coordinates.",
			Scope:       "markdown",
		},
	}

	fileCache := make(map[string][]string)

	for _, sym := range source.Symbols {
		// Determine scope
		isMarkdown := sym.Kind == "markdown"
		isCode := sym.Kind == store.SymFunction || sym.Kind == store.SymMethod

		if !isMarkdown && !isCode {
			continue
		}

		var contentBlock string
		var startLine int

		if isMarkdown {
			// For markdown symbols, the entire file content is kept in Sym.Doc
			contentBlock = sym.Doc
			startLine = 1
		} else {
			// Handle source code symbols
			lines, exists := fileCache[sym.File]
			if !exists {
				data, err := os.ReadFile(sym.File)
				if err != nil {
					fileCache[sym.File] = nil
					continue
				}
				lines = strings.Split(string(data), "\n")
				fileCache[sym.File] = lines
			}

			if lines == nil {
				continue
			}

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

			contentBlock = strings.Join(lines[startIdx:endIdx], "\n")
			startLine = sym.Line
		}

		// Scan relevant rules
		for _, rule := range rules {
			// Filter by scope
			if rule.Scope == "markdown" && !isMarkdown {
				continue
			}
			if rule.Scope == "code" && !isCode {
				continue
			}

			matches := rule.Pattern.FindAllStringIndex(contentBlock, -1)
			for _, match := range matches {
				beforeMatch := contentBlock[:match[0]]
				matchedOffset := strings.Count(beforeMatch, "\n")
				absoluteLine := startLine + matchedOffset

				matchedText := ""
				lines := strings.Split(contentBlock, "\n")
				if matchedOffset >= 0 && matchedOffset < len(lines) {
					matchedText = strings.TrimSpace(lines[matchedOffset])
				}

				symFullName := sym.Name
				if !isMarkdown && sym.Parent != "" {
					symFullName = sym.Parent + "." + sym.Name
				}

				// Avoid duplicate findings
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
