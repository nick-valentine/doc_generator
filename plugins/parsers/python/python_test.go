package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestPythonParser_Parse(t *testing.T) {
	srcCode := []byte(`
class AppConfig:
    """
    AppConfig stores system variables.
    @audience API INTERNAL
    @compatibility C
    """
    def __init__(self, port, host):
        self.port = port
        self.host = host

    def start(self):
        """
        Start initiates the server.
        @audience USER API
        """
        self.log()

    def log(self):
        pass

def new_config():
    """
    NewConfig creates standard config.
    @audience DEVELOPER
    """
    config = AppConfig(8080, "localhost")
    config.start()
    return config
`)

	pp := &PythonParser{}

	source := &store.Source{}
	err := pp.Parse("config.py", srcCode, source)
	if err != nil {
		t.Fatalf("expected no parsing error, got: %v", err)
	}

	// 1. Verify file registered
	file := source.GetFile("config.py")
	if file == nil {
		t.Errorf("expected config.py file to be registered")
	}

	// 2. Verify class
	structs := source.SearchSymbols("AppConfig")
	if len(structs) < 1 {
		t.Fatalf("expected to find AppConfig class symbol")
	}
	appConfigSym := structs[0]
	if appConfigSym.Kind != store.SymStruct {
		t.Errorf("expected AppConfig kind to be struct, got: %s", appConfigSym.Kind)
	}
	if len(appConfigSym.Audience) != 2 || appConfigSym.Audience[0] != "API" || appConfigSym.Audience[1] != "INTERNAL" {
		t.Errorf("expected audience [API INTERNAL], got: %v", appConfigSym.Audience)
	}

	// 3. Verify methods
	methods := source.GetStructMethods("AppConfig")
	if len(methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(methods))
	}

	// 4. Verify function
	funcs := source.SearchSymbols("new_config")
	if len(funcs) != 1 || funcs[0].Kind != store.SymFunction {
		t.Errorf("expected function new_config, got %v", funcs)
	}

	// 5. Verify Call Relations
	startCallees := source.GetCallees("AppConfig.start")
	if len(startCallees) != 1 || startCallees[0] != "self.log" {
		t.Errorf("expected AppConfig.start callees to be [self.log], got %v", startCallees)
	}
}
