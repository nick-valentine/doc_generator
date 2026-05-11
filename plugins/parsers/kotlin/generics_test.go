package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestKotlinParser_Generics(t *testing.T) {
	src := `
package com.example

class Box<T>(val value: T) {
	fun get(): T = value
}

data class Result<out T>(val data: T) {
	fun isSuccess(): Boolean = true
}

fun <E> processList(list: List<E>): Int {
	return list.size
}
`
	kp := &KotlinParser{}
	s := &store.Source{}
	
	err := kp.Parse("test.kt", []byte(src), s)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Assert Generics Separation for Class Names (Pure Identifier)
	boxSyms := s.SearchSymbols("Box")
	if len(boxSyms) == 0 || boxSyms[0].Name != "Box" {
		t.Fatalf("Expected Box struct, but could not find or parsed improperly")
	}
	resultSyms := s.SearchSymbols("Result")
	if len(resultSyms) == 0 || resultSyms[0].Name != "Result" {
		t.Fatalf("Expected Result struct from data class, got none")
	}

	// 2. Assert Parenting for Inner Definitions
	getSyms := s.SearchSymbols("get")
	if len(getSyms) == 0 || getSyms[0].Parent != "Box" {
		t.Errorf("Expected 'get' method to belong to Parent 'Box', got: %q", getSyms[0].Parent)
	}

	isSuccessSyms := s.SearchSymbols("isSuccess")
	if len(isSuccessSyms) == 0 || isSuccessSyms[0].Parent != "Result" {
		t.Errorf("Expected 'isSuccess' from data class body to belong to Parent 'Result', got: %q", isSuccessSyms[0].Parent)
	}

	// 3. Assert Global Scope Isolation (Crucial Verify for Physical Recalibration)
	procSyms := s.SearchSymbols("processList")
	if len(procSyms) == 0 {
		t.Fatalf("Failed to detect global generic processList function")
	}
	// Verify Parent IS EMPTY! (Extracted perfectly outside of preceding Data Class body!)
	if procSyms[0].Parent != "" {
		t.Errorf("CRITICAL: 'processList' erroneously absorbed by Parent %q. Bound recalibration FAILED.", procSyms[0].Parent)
	}
	if procSyms[0].Kind != store.SymFunction {
		t.Errorf("Expected 'processList' kind to be function, got: %s", procSyms[0].Kind)
	}
}
