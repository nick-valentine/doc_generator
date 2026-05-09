package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSource_AddFileAndSymbol(t *testing.T) {
	src := &Source{}
	src.AddFile("main.go")

	file := src.GetFile("main.go")
	if file == nil {
		t.Fatalf("expected file main.go to be added")
	}

	sym := Symbol{
		Name:     "Config",
		Kind:     SymStruct,
		File:     "main.go",
		Line:     10,
		Doc:      "Holds application configuration.",
		Audience: []string{"API"},
	}
	src.AddSymbol(sym)

	if len(src.Symbols) != 1 {
		t.Errorf("expected 1 symbol, got %d", len(src.Symbols))
	}
}

func TestSource_SearchSymbols(t *testing.T) {
	src := &Source{
		Symbols: []Symbol{
			{Name: "Config", Kind: SymStruct},
			{Name: "GetConfig", Kind: SymFunction},
			{Name: "parseLine", Kind: SymFunction},
		},
	}

	results := src.SearchSymbols("config")
	if len(results) != 2 {
		t.Errorf("expected 2 symbols matching 'config', got %d", len(results))
	}

	results = src.SearchSymbols("PARSE")
	if len(results) != 1 {
		t.Errorf("expected 1 symbol matching 'PARSE', got %d", len(results))
	}
}

func TestSource_FilterByAudience(t *testing.T) {
	src := &Source{
		Symbols: []Symbol{
			{Name: "Config", Kind: SymStruct, Audience: []string{"API", "INTERNAL"}},
			{Name: "getSecret", Kind: SymFunction, Audience: []string{"INTERNAL"}},
			{Name: "UserGuide", Kind: SymFunction, Audience: []string{"USER"}},
		},
	}

	api := src.FilterByAudience("API")
	if len(api) != 1 || api[0].Name != "Config" {
		t.Errorf("expected 1 symbol for audience API, got %d", len(api))
	}

	internal := src.FilterByAudience("internal")
	if len(internal) != 2 {
		t.Errorf("expected 2 symbols for audience INTERNAL, got %d", len(internal))
	}
}

func TestSource_GetStructFieldsAndMethods(t *testing.T) {
	src := &Source{
		Symbols: []Symbol{
			{Name: "Config", Kind: SymStruct},
			{Name: "Port", Kind: SymField, Parent: "Config"},
			{Name: "Host", Kind: SymField, Parent: "Config"},
			{Name: "Validate", Kind: SymMethod, Parent: "Config"},
			{Name: "OtherFunc", Kind: SymFunction},
		},
	}

	fields := src.GetStructFields("Config")
	if len(fields) != 2 {
		t.Errorf("expected 2 fields for struct Config, got %d", len(fields))
	}

	methods := src.GetStructMethods("Config")
	if len(methods) != 1 || methods[0].Name != "Validate" {
		t.Errorf("expected method Validate for struct Config, got %d", len(methods))
	}
}

func TestParseCoverage_MultiFormat(t *testing.T) {
	// 1. Test standard Go Cover format
	goCoverContent := `mode: set
main.go:10.5,15.20 3 1
main.go:16.5,20.20 2 0
`
	tempDir, err := os.MkdirTemp("", "cov_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCoverPath := filepath.Join(tempDir, "coverage.out")
	if err := os.WriteFile(goCoverPath, []byte(goCoverContent), 0644); err != nil {
		t.Fatalf("failed to write go cover test file: %v", err)
	}

	blocks, err := ParseCoverage(goCoverPath)
	if err != nil {
		t.Fatalf("failed to parse standard Go Cover: %v", err)
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].File != "main.go" || blocks[0].StartLine != 10 || blocks[0].EndLine != 15 || blocks[0].Count != 1 {
		t.Errorf("incorrect block parsing for Go Cover: %+v", blocks[0])
	}

	// 2. Test LCOV format
	lcovContent := `TN:
SF:pkg/store/source.go
DA:10,1
DA:11,0
end_of_record
`
	lcovPath := filepath.Join(tempDir, "coverage.info")
	if err := os.WriteFile(lcovPath, []byte(lcovContent), 0644); err != nil {
		t.Fatalf("failed to write lcov test file: %v", err)
	}

	blocks, err = ParseCoverage(lcovPath)
	if err != nil {
		t.Fatalf("failed to parse LCOV: %v", err)
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks for LCOV, got %d", len(blocks))
	}
	if blocks[0].File != "pkg/store/source.go" || blocks[0].StartLine != 10 || blocks[0].Count != 1 {
		t.Errorf("incorrect block parsing for LCOV: %+v", blocks[0])
	}

	// 3. Test CCOV format
	ccovContent := `main.go:30:5
main.go:35:0
`
	ccovPath := filepath.Join(tempDir, "coverage.ccov")
	if err := os.WriteFile(ccovPath, []byte(ccovContent), 0644); err != nil {
		t.Fatalf("failed to write ccov test file: %v", err)
	}

	blocks, err = ParseCoverage(ccovPath)
	if err != nil {
		t.Fatalf("failed to parse CCOV: %v", err)
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks for CCOV, got %d", len(blocks))
	}
	if blocks[0].File != "main.go" || blocks[0].StartLine != 30 || blocks[0].Count != 5 {
		t.Errorf("incorrect block parsing for CCOV: %+v", blocks[0])
	}
}
