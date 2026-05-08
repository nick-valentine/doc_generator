package store

import (
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
