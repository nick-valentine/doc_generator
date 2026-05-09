package main

import (
	"fmt"
	"os"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func main() {
	testFile := "pkg/analysis/network.go"
	content, err := os.ReadFile(testFile)
	if err != nil {
		panic(err)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_go.Language()))

	tree := parser.Parse(content, nil)
	defer tree.Close()

	root := tree.RootNode()
	fmt.Printf("Root child count: %d\n", root.ChildCount())

	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(uint(i))
		fmt.Printf("Child %d: Kind='%s' TextPrefix='%s'\n", i, child.Kind(), string(content[child.StartByte():child.StartByte()+20]))
	}
}
