package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestKotlinParser_Parse(t *testing.T) {
	srcCode := []byte(`
package com.example

/**
 * AppConfig stores system variables.
 * @audience API INTERNAL
 * @compatibility C
 */
class AppConfig {
	var port: Int = 8080
	var host: String = "localhost"

	/**
	 * Start initiates the server.
	 * @audience USER API
	 */
	fun start() {
		log()
	}

	fun log() {
	}
}

/**
 * NewConfig creates standard config.
 * @audience DEVELOPER
 */
fun newConfig(): AppConfig {
	val config = AppConfig()
	config.start()
	return config
}
`)

	kp := &KotlinParser{}

	source := &store.Source{}
	err := kp.Parse("AppConfig.kt", srcCode, source)
	if err != nil {
		t.Fatalf("expected no parsing error, got: %v", err)
	}

	// 1. Verify file registered
	file := source.GetFile("AppConfig.kt")
	if file == nil {
		t.Errorf("expected AppConfig.kt file to be registered")
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

	// 3. Verify methods & functions
	methods := source.GetStructMethods("AppConfig")
	if len(methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(methods))
	}

	// 4. Verify Call Relations
	startCallees := source.GetCallees("com.example.AppConfig.start")
	if len(startCallees) != 1 || startCallees[0] != "log" {
		t.Errorf("expected com.example.AppConfig.start callees to be [log], got %v", startCallees)
	}
}

func TestKotlinParser_Inheritance(t *testing.T) {
	srcCode := []byte(`
package com.example

class ChildService : BaseService, Initializable {
	fun process() {
	}
}
`)

	kp := &KotlinParser{}
	source := &store.Source{}
	err := kp.Parse("ChildService.kt", srcCode, source)
	if err != nil {
		t.Fatalf("expected no parsing error, got: %v", err)
	}

	syms := source.SearchSymbols("ChildService")
	if len(syms) < 1 {
		t.Fatalf("expected to find ChildService symbol")
	}
	childSym := syms[0]
	if len(childSym.Relations) != 2 {
		t.Errorf("expected 2 relations, got %d: %v", len(childSym.Relations), childSym.Relations)
	} else {
		if childSym.Relations[0] != "BaseService" || childSym.Relations[1] != "Initializable" {
			t.Errorf("expected relations [BaseService Initializable], got %v", childSym.Relations)
		}
	}
}
