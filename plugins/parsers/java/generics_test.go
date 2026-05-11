package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestJavaParser_Generics(t *testing.T) {
	src := `
package com.example;
import java.util.List;

public class GenericBox<T> {
	private List<String> items;
	private T data;

	public List<String> getItems() {
		return items;
	}

	public void setData(T val) {
		this.data = val;
	}

	public <E> E process(E input) {
		return input;
	}
}
`
	jp := &JavaParser{}
	s := &store.Source{}
	
	err := jp.Parse("test.java", []byte(src), s)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Assert Generics for Field Types
	itemsSyms := s.SearchSymbols("items")
	if len(itemsSyms) == 0 || itemsSyms[0].Type != "List<String>" {
		t.Errorf("Expected items field to have Type 'List<String>', got: %q", itemsSyms[0].Type)
	}
	dataSyms := s.SearchSymbols("data")
	if len(dataSyms) == 0 || dataSyms[0].Type != "T" {
		t.Errorf("Expected data field to have Type 'T', got: %q", dataSyms[0].Type)
	}

	// 2. Assert Generics for Return Types
	getItemsSyms := s.SearchSymbols("getItems")
	if len(getItemsSyms) == 0 || getItemsSyms[0].Returns != "List<String>" {
		t.Errorf("Expected getItems to have Returns 'List<String>', got: %q", getItemsSyms[0].Returns)
	}

	// 3. Assert Generics for Method Parameters
	setDataSyms := s.SearchSymbols("setData")
	if len(setDataSyms) == 0 || setDataSyms[0].Params != "(T val)" {
		t.Errorf("Expected setData to have Params '(T val)', got: %q", setDataSyms[0].Params)
	}

	// 4. Assert Advanced Parametric Method
	processSyms := s.SearchSymbols("process")
	if len(processSyms) == 0 {
		t.Fatalf("Failed to find 'process' method")
	}
	if processSyms[0].Returns != "E" || processSyms[0].Params != "(E input)" {
		t.Errorf("Expected process method to have Returns 'E' and Params '(E input)', got: Returns=%q Params=%q", processSyms[0].Returns, processSyms[0].Params)
	}
}
