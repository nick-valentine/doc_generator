package main

import (
	"fmt"
	"os"
	"plugin"
	"doc_generator/pkg/store"
)

func main() {
	plug, err := plugin.Open("plugins/parsers/odin_parser.so")
	if err != nil {
		panic(err)
	}

	symParser, err := plug.Lookup("Parser")
	if err != nil {
		panic(err)
	}

	p := symParser.(*store.Parser)

	testFile := "../beholderFPS/cicd/odin/core/container/queue/queue.odin"
	content, err := os.ReadFile(testFile)
	if err != nil {
		// Fallback to generic inline content test if file doesn't exist locally
		fmt.Println("File missing, using inline mock.")
		content = []byte("package queue\n\nQueue :: struct($T: typeid) {\n\tdata: [dynamic]T,\n}\n")
	}

	var s store.Source
	err = (*p).Parse(testFile, content, &s)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Tested file: %s\n", testFile)
	fmt.Printf("Total symbols found: %d\n", len(s.Symbols))
	for _, sym := range s.Symbols {
		fmt.Printf("-> Found Symbol: %s [%s]\n", sym.Name, sym.Kind)
	}
}
