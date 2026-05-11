package main

import (
	"fmt"
	"testing"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func TestJsAsync(t *testing.T) {
	src := []byte(`
	async function test() {}
	const x = async () => {};
	class C { async doWork() {} }
	`)
	parser := tree_sitter.NewParser()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language()))
	tree := parser.Parse(src, nil)
	root := tree.RootNode()
	
	var walk func(*tree_sitter.Node)
	walk = func(n *tree_sitter.Node) {
		if n == nil { return }
		if n.Kind() == "async" {
			fmt.Printf("Found async keyword under parent kind: %s\n", n.Parent().Kind())
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(uint(i)))
		}
	}
	walk(root)
}
