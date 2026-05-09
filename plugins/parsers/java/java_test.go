package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestJavaParser_Parse(t *testing.T) {
	srcCode := []byte(`
package com.example;

/**
 * AppConfig stores system variables.
 * @audience API INTERNAL
 * @compatibility C
 */
public class AppConfig {
	/**
	 * Port number of server.
	 * @audience INTERNAL
	 */
	private int port;
	private String host;

	/**
	 * Start initiates the server.
	 * @audience USER API
	 */
	public void start() {
		log();
	}

	public void log() {
	}

	/**
	 * NewConfig creates standard config.
	 * @audience DEVELOPER
	 */
	public static AppConfig newConfig() {
		AppConfig config = new AppConfig();
		config.start();
		return config;
	}
}
`)

	jp := &JavaParser{}

	source := &store.Source{}
	err := jp.Parse("AppConfig.java", srcCode, source)
	if err != nil {
		t.Fatalf("expected no parsing error, got: %v", err)
	}

	// 1. Verify file registered
	file := source.GetFile("AppConfig.java")
	if file == nil {
		t.Errorf("expected AppConfig.java file to be registered")
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

	// 3. Verify fields
	fields := source.GetStructFields("AppConfig")
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}

	// 4. Verify methods
	methods := source.GetStructMethods("AppConfig")
	if len(methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(methods))
	}

	// 5. Verify Call Relations
	startCallees := source.GetCallees("com.example.AppConfig.start")
	if len(startCallees) != 1 || startCallees[0] != "log" {
		t.Errorf("expected com.example.AppConfig.start callees to be [log], got %v", startCallees)
	}
}
