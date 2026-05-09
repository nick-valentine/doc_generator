package main

import (
	"fmt"
	"os"
	"plugin"
	"doc_generator/pkg/store"
)

func main() {
	plug, err := plugin.Open("plugins/parsers/go_parser.so")
	if err != nil {
		panic(err)
	}

	symParser, err := plug.Lookup("Parser")
	if err != nil {
		panic(err)
	}

	p := symParser.(*store.Parser)

	testFile := "pkg/analysis/network.go"
	content, err := os.ReadFile(testFile)
	if err != nil {
		panic(err)
	}

	var s store.Source
	err = (*p).Parse(testFile, content, &s)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Tested file: %s\n", testFile)
	fmt.Printf("Total symbols found: %d\n", len(s.Symbols))
	for i, sym := range s.Symbols {
		if i > 5 { break }
		fmt.Printf("Symbol: %s | Kind: %s | Package: '%s'\n", sym.Name, sym.Kind, sym.Package)
	}
}
