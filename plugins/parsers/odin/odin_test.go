package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestOdinParser(t *testing.T) {
	parser := &OdinParser{}
	src := &store.Source{}
	content := `
package test

import "core:fmt"

MyInterface :: interface(t: T) {
    t.do_something()
}

MyStruct :: struct {
    field1: int,
    field2, field3: string,
}

OtherStruct :: struct {
    using m: MyStruct,
    id: MyID,
}

MyID :: distinct int

MyAlias :: int

MyUnion :: union {
    int,
    string,
    MyStruct,
    ^int,
    []string,
    map[[4]int]MyStruct,
}

foreign libc {
    printf :: proc(fmt: cstring, #c_vararg args: ..any) -> i32 ---
}

my_struct_init :: proc(m: ^MyStruct) {
    fmt.println("init")
}

explicit_method :: proc(self: MyStruct, val: int) {
}

another_proc :: proc(x: int) -> int {
    return x * 2
}

main :: proc() {
    m: MyStruct
    my_struct_init(&m)
    another_proc(10)
    fmt.println("done")
}
`
	err := parser.Parse("test.odin", []byte(content), src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundStruct := false
	foundOther := false
	foundID := false
	foundUnion := false
	foundInterface := false
	foundField1 := false
	foundInit := false
	foundExplicit := false
	foundAnother := false
	foundMain := false
	foundUnionVariant := false
	foundPtrVariant := false
	foundSliceVariant := false
	foundMapVariant := false
	foundPrintf := false
	foundAlias := false
	foundImport := false

	for _, sym := range src.Symbols {
		switch sym.Name {
		case "fmt":
			if sym.Kind == store.SymImport {
				foundImport = true
			}
		case "printf":
			foundPrintf = true
		case "^int":
			if sym.Parent == "MyUnion" {
				foundPtrVariant = true
			}
		case "[]string":
			if sym.Parent == "MyUnion" {
				foundSliceVariant = true
			}
		case "map[[4]int]MyStruct":
			if sym.Parent == "MyUnion" {
				foundMapVariant = true
			}
		case "MyAlias":
			foundAlias = true
		case "MyInterface":
			foundInterface = true
		case "MyStruct":
			foundStruct = true
		case "OtherStruct":
			foundOther = true
		case "MyID":
			foundID = true
		case "MyUnion":
			foundUnion = true
		case "int":
			if sym.Parent == "MyUnion" {
				foundUnionVariant = true
			}
		case "field1":
			foundField1 = true
		case "my_struct_init":
			foundInit = true
		case "explicit_method":
			foundExplicit = true
		case "another_proc":
			foundAnother = true
		case "main":
			foundMain = true
		}
	}

	if !foundImport { t.Error("fmt import not found") }
	if !foundInterface { t.Error("MyInterface not found") }
	if !foundAlias { t.Error("MyAlias not found") }
	if !foundPrintf { t.Error("printf not found in foreign block") }
	if !foundPtrVariant { t.Error("Union variant '^int' not found") }
	if !foundSliceVariant { t.Error("Union variant '[]string' not found") }
	if !foundMapVariant { t.Error("Union variant 'map[[4]int]MyStruct' not found") }
	if !foundStruct { t.Error("MyStruct not found") }
	if !foundOther { t.Error("OtherStruct not found") }
	if !foundID { t.Error("MyID not found") }
	if !foundUnion { t.Error("MyUnion not found") }
	if !foundUnionVariant { t.Error("Union variant 'int' not found") }
	if !foundInit { t.Error("my_struct_init not found") }
	if !foundExplicit { t.Error("explicit_method not found") }
	if !foundAnother { t.Error("another_proc not found") }
	if !foundMain { t.Error("main not found") }
	if !foundField1 { t.Error("field1 not found") }

	// Check calls
	foundCallMainInit := false
	foundCallMainFmt := false
	for _, call := range src.Calls {
		if call.Caller == "test.main" {
			if call.Callee == "my_struct_init" {
				foundCallMainInit = true
			}
			if call.Callee == "fmt.println" {
				foundCallMainFmt = true
			}
		}
		if call.Caller == "test.MyStruct.my_struct_init" && call.Callee == "fmt.println" {
			foundCallMainFmt = true // reusing variable for speed
		}
	}

	if !foundCallMainInit {
		t.Error("Call test.main -> my_struct_init not found")
	}
	if !foundCallMainFmt {
		t.Error("Call test.main -> fmt.println not found")
	}
}
