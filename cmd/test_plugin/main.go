package main

import (
	"fmt"
	"plugin"
)

func main() {
	// Load plugins in the same order as cmd/generate/main.go
	for _, path := range []string{
		"plugins/parsers/go_parser.so",
		"plugins/parsers/markdown_parser.so",
		"plugins/parsers/odin_parser.so",
		"plugins/generators/html_generator.so",
		"plugins/generators/text_generator.so",
	} {
		p, err := plugin.Open(path)
		if err != nil {
			fmt.Printf("ERROR loading plugin %s: %v\n", path, err)
			continue
		}
		fmt.Printf("Loaded %s: %v\n", path, p)
	}
}
