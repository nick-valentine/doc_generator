package main

import (
	"doc_generator/pkg/store"
	"path/filepath"
	"strings"
)

// MarkdownParser parses .md files and registers them in the store.
type MarkdownParser struct{}

// Parse reads a markdown file and extracts its title (first H1) and content as a symbol.
func (mp *MarkdownParser) Parse(filePath string, fileContent []byte, source *store.Source) error {
	source.AddFile(filePath)

	content := string(fileContent)
	lines := strings.Split(content, "\n")
	
	// Extract first H1 header as the symbol Name
	symbolName := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			symbolName = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			break
		}
	}

	if symbolName == "" {
		symbolName = filepath.Base(filePath)
	}

	source.AddSymbol(store.Symbol{
		Name:     symbolName,
		Kind:     "markdown",
		File:     filePath,
		Doc:      content,
		Audience: []string{"USER", "DEVELOPER"},
	})

	return nil
}

// Parser is the exported parser implementation
var Parser store.Parser = &MarkdownParser{}

// Extensions is the list of file extensions this parser handles
var Extensions = []string{".md"}
