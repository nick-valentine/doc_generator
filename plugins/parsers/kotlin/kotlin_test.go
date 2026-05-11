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

	// 4. Verify Fields extracted
	fields := source.GetStructFields("AppConfig")
	if len(fields) != 2 {
		t.Errorf("expected 2 fields (port, host), got %d", len(fields))
	} else {
		foundPort := false
		foundHost := false
		for _, f := range fields {
			if f.Name == "port" { foundPort = true }
			if f.Name == "host" { foundHost = true }
		}
		if !foundPort || !foundHost {
			t.Errorf("Missing port or host field in extracted fields")
		}
	}

	// 5. Verify Call Relations
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
func TestKotlinParser_AdvancedFunctionBodies(t *testing.T) {
	srcCode := []byte(`
package com.example

class ComplexService {
	@Override
	fun handleRequest(req: String): Response {
		internalCall()
		return Response()
	}

	private fun internalCall() {
		if (true) {
			moreComplexity()
		}
	}

	fun expressionBody() = 42

	override fun test(req: String): Int = 54
}
`)

	kp := &KotlinParser{}
	source := &store.Source{}
	err := kp.Parse("ComplexService.kt", srcCode, source)
	if err != nil {
		t.Fatalf("expected no parsing error, got: %v", err)
	}

	// Verify handleRequest caught despite @Override
	handleSyms := source.SearchSymbols("handleRequest")
	if len(handleSyms) < 1 {
		t.Fatalf("expected to find handleRequest method symbol")
	}
	
	// Verify return type captured
	if handleSyms[0].Returns != "Response" {
		t.Errorf("expected return type 'Response', got '%s'", handleSyms[0].Returns)
	}

	// Verify internalCall caught despite private modifier
	internalSyms := source.SearchSymbols("internalCall")
	if len(internalSyms) < 1 {
		t.Fatalf("expected to find internalCall method symbol")
	}
	
	// Verify complexity > 1 due to IF statement inside internalCall
	if internalSyms[0].Complexity <= 1 {
		t.Errorf("expected complexity > 1, got %d", internalSyms[0].Complexity)
	}

	// Verify call graph linking
	callees := source.GetCallees("com.example.ComplexService.handleRequest")
	found := false
	for _, c := range callees {
		if c == "internalCall" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected call to internalCall in handleRequest, got callees: %v", callees)
	}

	// Verify 'test' method with expression body
	testSyms := source.SearchSymbols("test")
	if len(testSyms) < 1 {
		t.Fatalf("expected to find 'test' method with expression body symbol")
	}
	if testSyms[0].Returns != "Int" {
		t.Errorf("expected return type 'Int' for expression body, got '%s'", testSyms[0].Returns)
	}
}
func TestKotlinParser_ExtremeEdgeCases(t *testing.T) {
	srcCode := []byte(`
package tech.example

data class UserProfile(val id: String, val name: String)

class Database(connString: String) : Resource(connString), Closeable {
	companion object {
		fun createDefault(): Database {
			return Database("default")
		}
	}

	fun query() {
	}
}
`)

	kp := &KotlinParser{}
	source := &store.Source{}
	err := kp.Parse("Extreme.kt", srcCode, source)
	if err != nil {
		t.Fatalf("parsing error: %v", err)
	}

	// 1. Verify Data Class detected
	userSyms := source.SearchSymbols("UserProfile")
	if len(userSyms) < 1 {
		t.Errorf("Failed to detect data class UserProfile")
	} else if userSyms[0].Kind != store.SymStruct {
		t.Errorf("UserProfile should be Struct, got %s", userSyms[0].Kind)
	}
	// Verify properties from primary constructor
	userFields := source.GetStructFields("UserProfile")
	if len(userFields) != 2 {
		t.Errorf("Expected 2 fields from data class constructor (id, name), got %d", len(userFields))
	} else {
		hasId := false
		for _, f := range userFields { if f.Name == "id" { hasId = true } }
		if !hasId { t.Errorf("Missing 'id' property in UserProfile") }
	}

	// 2. Verify Inheritance with constructor arguments handled correctly
	dbSyms := source.SearchSymbols("Database")
	if len(dbSyms) < 1 {
		t.Fatalf("Failed to detect Database class")
	}
	db := dbSyms[0]
	// extractKotlinRelations should correctly prune "(connString)" and return base classes
	foundResource := false
	foundCloseable := false
	for _, rel := range db.Relations {
		if rel == "Resource" { foundResource = true }
		if rel == "Closeable" { foundCloseable = true }
	}
	if !foundResource || !foundCloseable {
		t.Errorf("Database relations incorrect, found: %v", db.Relations)
	}

	// 3. Verify Companion Object listed
	compSyms := source.SearchSymbols("Companion")
	if len(compSyms) < 1 {
		t.Errorf("Failed to detect Companion object")
	}

	// 4. Verify function inside Companion Object detected
	createSyms := source.SearchSymbols("createDefault")
	if len(createSyms) < 1 {
		t.Errorf("Failed to detect method createDefault inside companion")
	}
}

func TestKotlinParser_Async(t *testing.T) {
	srcCode := []byte(`
package com.async
import kotlinx.coroutines.*

suspend fun globalJob(): String {
	delay(1000)
	return "done"
}

class Worker {
	suspend fun doWork() {
		println("Working...")
	}
}
`)
	kp := &KotlinParser{}
	source := &store.Source{}
	err := kp.Parse("Async.kt", srcCode, source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Check top level suspend fun
	jSyms := source.SearchSymbols("globalJob")
	if len(jSyms) == 0 {
		t.Errorf("globalJob not found")
	} else if !jSyms[0].IsAsync {
		t.Errorf("globalJob should be IsAsync=true")
	}

	// Check class level suspend method
	wSyms := source.SearchSymbols("doWork")
	if len(wSyms) == 0 {
		t.Errorf("doWork method not found")
	} else {
		found := false
		for _, s := range wSyms {
			if s.Parent == "Worker" {
				found = true
				if !s.IsAsync {
					t.Errorf("Worker.doWork should be IsAsync=true")
				}
			}
		}
		if !found {
			t.Errorf("Worker.doWork not found in symbols")
		}
	}
}

func TestKotlinParser_AsyncExpressionBody(t *testing.T) {
	srcCode := []byte(`
package com.async
import kotlinx.coroutines.*

suspend fun performExpression() = coroutineScope {
	"Result"
}
`)
	kp := &KotlinParser{}
	source := &store.Source{}
	err := kp.Parse("Expr.kt", srcCode, source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// 1. Verify symbol found
	syms := source.SearchSymbols("performExpression")
	if len(syms) == 0 {
		t.Fatalf("performExpression not found")
	}
	sym := syms[0]

	// 2. Verify IsAsync still TRUE despite expression body instead of block body
	if !sym.IsAsync {
		t.Errorf("performExpression with expression body should be IsAsync=true")
	}

	// 3. Verify Call Relation captured from the expression RHS (coroutineScope)
	fullName := "com.async.performExpression"
	callees := source.GetCallees(fullName)
	foundScope := false
	for _, c := range callees {
		if c == "coroutineScope" {
			foundScope = true
			break
		}
	}
	if !foundScope {
		t.Errorf("Expected call to 'coroutineScope' in expression body, got callees: %v", callees)
	}
}
