package main

import (
	"fmt"
	"strings"
	"testing"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_kotlin "github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go"
)

func TestCheckSuspend(t *testing.T) {
	src := []byte(`suspend fun processData() {}`)
	parser := tree_sitter.NewParser()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_kotlin.Language()))
	tree := parser.Parse(src, nil)
	root := tree.RootNode()
	
	var walk func(*tree_sitter.Node, int)
	walk = func(node *tree_sitter.Node, d int) {
		if node == nil { return }
		fmt.Printf("%s%s: %q\n", strings.Repeat(" ", d*2), node.Kind(), string(src[node.StartByte():node.EndByte()]))
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(uint(i)), d+1)
		}
	}
	walk(root, 0)
}
