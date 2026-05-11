package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestGoParser_Parse(t *testing.T) {
	srcCode := []byte(`
package config

// AppConfig stores system variables.
// @audience API INTERNAL
// @compatibility C
type AppConfig struct {
	// Port number of server.
	// @audience INTERNAL
	Port int
	Host string
}

// Start initiates the server.
// @audience USER API
func (c *AppConfig) Start() {
	c.Log()
}

func (c *AppConfig) Log() {
}

// NewConfig creates standard config.
// @audience DEVELOPER
func NewConfig() *AppConfig {
	config := &AppConfig{}
	config.Start()
	return config
}
`)

	gp := &GoParser{}

	source := &store.Source{}
	err := gp.Parse("config.go", srcCode, source)
	if err != nil {
		t.Fatalf("expected no parsing error, got: %v", err)
	}

	// 1. Verify file registered
	file := source.GetFile("config.go")
	if file == nil {
		t.Errorf("expected config.go file to be registered")
	}

	// 2. Verify struct
	structs := source.SearchSymbols("AppConfig")
	if len(structs) < 1 {
		t.Fatalf("expected to find AppConfig struct symbol")
	}
	appConfigSym := structs[0]
	if appConfigSym.Kind != store.SymStruct {
		t.Errorf("expected AppConfig kind to be struct, got: %s", appConfigSym.Kind)
	}
	if len(appConfigSym.Audience) != 2 || appConfigSym.Audience[0] != "API" || appConfigSym.Audience[1] != "INTERNAL" {
		t.Errorf("expected audience [API INTERNAL], got: %v", appConfigSym.Audience)
	}
	if len(appConfigSym.Compatibility) != 1 || appConfigSym.Compatibility[0] != "C" {
		t.Errorf("expected compatibility [C], got: %v", appConfigSym.Compatibility)
	}

	// 3. Verify fields
	fields := source.GetStructFields("AppConfig")
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
	var portField store.Symbol
	for _, f := range fields {
		if f.Name == "Port" {
			portField = f
		}
	}
	if portField.Name != "Port" {
		t.Errorf("expected to find Port field")
	}
	if len(portField.Audience) != 1 || portField.Audience[0] != "INTERNAL" {
		t.Errorf("expected Port field audience [INTERNAL], got: %v", portField.Audience)
	}

	// 4. Verify methods
	methods := source.GetStructMethods("AppConfig")
	if len(methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(methods))
	}
	var startMethod store.Symbol
	for _, m := range methods {
		if m.Name == "Start" {
			startMethod = m
		}
	}
	if startMethod.Name != "Start" {
		t.Errorf("expected to find Start method")
	}
	if len(startMethod.Audience) != 2 || startMethod.Audience[0] != "USER" || startMethod.Audience[1] != "API" {
		t.Errorf("expected Start audience [USER API], got: %v", startMethod.Audience)
	}

	// 5. Verify function
	funcs := source.SearchSymbols("NewConfig")
	if len(funcs) != 1 || funcs[0].Kind != store.SymFunction {
		t.Errorf("expected function NewConfig, got %v", funcs)
	}
	newConfigFunc := funcs[0]
	if len(newConfigFunc.Audience) != 1 || newConfigFunc.Audience[0] != "DEVELOPER" {
		t.Errorf("expected NewConfig audience [DEVELOPER], got: %v", newConfigFunc.Audience)
	}

	// 6. Verify Call Relations
	startCallees := source.GetCallees("config.AppConfig.Start")
	if len(startCallees) != 1 || startCallees[0] != "c.Log" {
		t.Errorf("expected config.AppConfig.Start callees to be [c.Log], got %v", startCallees)
	}

	newConfigCallees := source.GetCallees("config.NewConfig")
	if len(newConfigCallees) != 1 || newConfigCallees[0] != "config.Start" {
		t.Errorf("expected config.NewConfig callees to be [config.Start], got %v", newConfigCallees)
	}
}

func TestGoParser_Async(t *testing.T) {
	srcCode := []byte(`
package async
func RunTask() {
	go func() {
		println("doing background task")
	}()
}

func ProcessStream(ch chan int) {
	select {
	case <-ch:
		return
	}
}
`)

	gp := &GoParser{}
	source := &store.Source{}
	err := gp.Parse("async.go", srcCode, source)
	if err != nil {
		t.Fatalf("parsing error: %v", err)
	}

	// Check RunTask (uses 'go' statement)
	taskSyms := source.SearchSymbols("RunTask")
	if len(taskSyms) == 0 {
		t.Errorf("RunTask not found")
	} else if !taskSyms[0].IsAsync {
		t.Errorf("RunTask should be detected as Async due to 'go' statement")
	}

	// Check ProcessStream (uses channel logic or select)
	streamSyms := source.SearchSymbols("ProcessStream")
	if len(streamSyms) == 0 {
		t.Errorf("ProcessStream not found")
	} else if !streamSyms[0].IsAsync {
		t.Errorf("ProcessStream should be detected as Async due to select/channels")
	}
}
