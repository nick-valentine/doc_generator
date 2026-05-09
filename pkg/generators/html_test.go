package generators

import (
	"doc_generator/pkg/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTMLGenerator_Generate(t *testing.T) {
	source := &store.Source{
		Files: []store.File{
			{Name: "config.go"},
		},
		Symbols: []store.Symbol{
			{
				Name:     "AppConfig",
				Kind:     store.SymStruct,
				File:     "config.go",
				Line:     10,
				Doc:      "AppConfig holds general application configuration.",
				Audience: []string{"API"},
			},
		},
	}

	hg := &HTMLGenerator{}
	tempDir, err := os.MkdirTemp("", "html_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = hg.Generate(source, tempDir)
	if err != nil {
		t.Fatalf("expected no HTML generation error, got %v", err)
	}

	contentBytes, err := os.ReadFile(filepath.Join(tempDir, "index.html"))
	if err != nil {
		t.Fatalf("expected index.html to be written, got %v", err)
	}
	htmlContent := string(contentBytes)

	// Verify crucial HTML structures and symbols are present
	if !strings.Contains(htmlContent, "<!DOCTYPE html>") {
		t.Errorf("expected HTML5 doctype declaration")
	}
	if !strings.Contains(htmlContent, "AppConfig") {
		t.Errorf("expected AppConfig struct to be rendered in HTML")
	}
	if !strings.Contains(htmlContent, "tag-aud") {
		t.Errorf("expected audience badges to be rendered")
	}
	if !strings.Contains(htmlContent, "lightbox") {
		t.Errorf("expected interactive lightbox scripts/styles")
	}
}
