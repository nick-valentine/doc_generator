package main

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func main() {
	code := `
import React from 'react';

type Props = {
    title: string;
    onClick: () => void;
}

export class MyComp extends React.Component<Props> {
    render(): JSX.Element { 
        return <div>Hello</div> 
    }
}

export const FuncComp = ({title}: Props): JSX.Element => {
    return <div>{title}</div>;
}
`
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language()))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	printAST(tree.RootNode(), 0, []byte(code))
}

func printAST(n *tree_sitter.Node, depth int, src []byte) {
	if n == nil { return }
	indent := strings.Repeat("  ", depth)
	text := string(src[n.StartByte():n.EndByte()])
	if len(text) > 30 { text = text[:30] + "..." }
	text = strings.ReplaceAll(text, "\n", " ")
	fmt.Printf("%s%s: %q\n", indent, n.Kind(), text)
	for i := uint(0); i < n.ChildCount(); i++ {
		printAST(n.Child(i), depth+1, src)
	}
}
