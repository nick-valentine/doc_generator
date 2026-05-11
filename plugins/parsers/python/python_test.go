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
func TestPythonParser_NewFeatures(t *testing.T) {
	srcCode := []byte(`
class BaseClass:
    pass

class ChildClass(BaseClass, mixin.Other):
    def process(self, val: int) -> str:
        if val > 10:
            return "big"
        return "small"
`)

	pp := &PythonParser{}
	source := &store.Source{}
	err := pp.Parse("features.py", srcCode, source)
	if err != nil {
		t.Fatalf("expected no parsing error, got: %v", err)
	}

	// 1. Verify relations
	childSyms := source.SearchSymbols("ChildClass")
	if len(childSyms) < 1 {
		t.Fatalf("ChildClass not found")
	}
	child := childSyms[0]
	if len(child.Relations) != 2 {
		t.Errorf("expected 2 relations, got %d: %v", len(child.Relations), child.Relations)
	} else {
		if child.Relations[0] != "BaseClass" || child.Relations[1] != "mixin.Other" {
			t.Errorf("unexpected relations: %v", child.Relations)
		}
	}

	// 2. Verify return hints
	procSyms := source.SearchSymbols("process")
	if len(procSyms) < 1 {
		t.Fatalf("process method not found")
	}
	proc := procSyms[0]
	if proc.Returns != "str" {
		t.Errorf("expected returns 'str', got '%s'", proc.Returns)
	}

	// 3. Verify complexity includes the 'if' statement
	if proc.Complexity <= 1 {
		t.Errorf("expected complexity > 1, got %d", proc.Complexity)
	}
}

func TestPythonParser_Async(t *testing.T) {
	srcCode := []byte(`
async def global_async_func():
    await something()
    return 42

class AsyncHandler:
    async def load_data(self):
        pass
`)
	pp := &PythonParser{}
	source := &store.Source{}
	err := pp.Parse("async.py", srcCode, source)
	if err != nil {
		t.Fatalf("failed parsing python async source: %v", err)
	}

	// Check global async function
	gSyms := source.SearchSymbols("global_async_func")
	if len(gSyms) == 0 {
		t.Errorf("global_async_func not found")
	} else if !gSyms[0].IsAsync {
		t.Errorf("global_async_func should have IsAsync=true")
	}

	// Check async class method
	mSyms := source.SearchSymbols("load_data")
	if len(mSyms) == 0 {
		t.Errorf("load_data method not found")
	} else {
		found := false
		for _, s := range mSyms {
			if s.Parent == "AsyncHandler" {
				found = true
				if !s.IsAsync {
					t.Errorf("AsyncHandler.load_data should have IsAsync=true")
				}
			}
		}
		if !found {
			t.Errorf("AsyncHandler.load_data not found in method symbols")
		}
	}
}
