package main

import (
	"fmt"
	"testing"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

func TestPyAsync(t *testing.T) {
	src := []byte(`async def my_task(): pass`)
	parser := tree_sitter.NewParser()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_python.Language()))
	tree := parser.Parse(src, nil)
	root := tree.RootNode()
	
	var walk func(*tree_sitter.Node, int)
	walk = func(n *tree_sitter.Node, d int) {
		if n == nil { return }
		fmt.Printf("%*sKind: %s\n", d*2, "", n.Kind())
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(uint(i)), d+1)
		}
	}
	walk(root, 0)
}
