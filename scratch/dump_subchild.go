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
	child := root.Child(uint(0)) // The package_clause

	fmt.Printf("Package Clause Kind: %s\n", child.Kind())
	fmt.Printf("Sub-child count: %d\n", child.ChildCount())

	for i := 0; i < int(child.ChildCount()); i++ {
		sc := child.Child(uint(i))
		fieldName := child.FieldNameForChild(uint32(i))
		fmt.Printf("  Child %d: Kind='%s' FieldName='%s' Text='%s'\n", i, sc.Kind(), fieldName, string(content[sc.StartByte():sc.EndByte()]))
	}

	nn := child.ChildByFieldName("name")
	if nn == nil {
		fmt.Println("ChildByFieldName('name') returned NIL!")
	} else {
		fmt.Printf("ChildByFieldName('name') returned Kind='%s'\n", nn.Kind())
	}
}
