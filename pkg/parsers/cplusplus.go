package parsers

import (
	"doc_generator/pkg/store"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
)

func BreadthFirstWalk(node *tree_sitter.Node, op func(*tree_sitter.Node)) {
	cursor := node.Walk()
	defer cursor.Close()
	BreadthFirstWalkHelper(cursor, op)
}

func BreadthFirstWalkHelper(cursor *tree_sitter.TreeCursor, op func(*tree_sitter.Node)) {
	op(cursor.Node())
	if cursor.GotoFirstChild() {
		BreadthFirstWalkHelper(cursor, op)
		cursor.GotoParent()
	}
	for cursor.GotoNextSibling() {
		BreadthFirstWalkHelper(cursor, op)
	}
}

func WalkSiblings(cursor *tree_sitter.TreeCursor, op func(*tree_sitter.TreeCursor)) {
	op(cursor)
	for cursor.GotoNextSibling() {
		op(cursor)
	}
}

func NodeSource(source []byte, node *tree_sitter.Node) string {
	return string(source[node.StartByte():node.EndByte()])
}

type CPlusPlus struct {
	FileName string
	File     []byte
}

func (cpp *CPlusPlus) Parse(source *store.Source) {
	source.Files = append(source.Files, store.File{
		Name: cpp.FileName,
	})

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_cpp.Language()))

	tree := parser.Parse(cpp.File, nil)
	defer tree.Close()

	root := tree.RootNode()
	cursor := root.Walk()
	defer cursor.Close()
	if !cursor.GotoFirstChild() {
		return
	}

	cpp.ParseGlobalDefinitions(cursor, source)

	//BreadthFirstWalk(root, func(node tree_sitter.Node) {
	//	fmt.Println(node.Kind())
	//})
}

func (cpp *CPlusPlus) ParseGlobalDefinitions(cursor *tree_sitter.TreeCursor, source *store.Source) {
	file := source.GetFile(cpp.FileName)
	WalkSiblings(cursor, func(c *tree_sitter.TreeCursor) {
		node := c.Node()
		//fmt.Println(node.Kind())
		switch node.Kind() {
		case "preproc_include":
			cursor.GotoFirstChild()
			cpp.ParsePreprocInclude(cursor, file)
			cursor.GotoParent()
		}
	})
}

func (cpp *CPlusPlus) ParsePreprocInclude(cursor *tree_sitter.TreeCursor, file *store.File) {
	WalkSiblings(cursor, func(c *tree_sitter.TreeCursor) {
		node := c.Node()
		switch node.Kind() {
		case "string_literal":
			file.AddFileImport(store.File{Name: cpp.StringContentFromStringLiteral(cursor)})
		case "system_lib_string":
			file.AddFileImport(store.File{Name: NodeSource(cpp.File, node)})
		}
	})
}

func (cpp *CPlusPlus) StringContentFromStringLiteral(cursor *tree_sitter.TreeCursor) string {
	cursor.GotoFirstChild()
	out := ""
	WalkSiblings(cursor, func(c *tree_sitter.TreeCursor) {
		if cursor.Node().Kind() == "string_content" {
			out = NodeSource(cpp.File, cursor.Node())
		}
	})
	cursor.GotoParent()
	return out
}
