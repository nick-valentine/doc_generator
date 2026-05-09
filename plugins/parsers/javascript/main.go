package main

import (
	"doc_generator/pkg/parsers/frontend"
	"doc_generator/pkg/store"
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

// Parser is the exported parser implementation
var Parser store.Parser = &JavascriptParser{}

// Extensions defines supported JavaScript and TypeScript variants
var Extensions = []string{".js", ".jsx", ".ts", ".tsx"}

// NodeSource extracts the raw source string corresponding to a Node.
func NodeSource(source []byte, node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}

// JavascriptParser implements extraction for both standard JS and React/TS via custom preprocessing.
type JavascriptParser struct {
	FileName    string
	RawSource   []byte
	CleanSource []byte
	Package     string
}

// Parse performs initial cleaning then hands over to standard Tree-Sitter.
func (jp *JavascriptParser) Parse(filePath string, fileContent []byte, source *store.Source) error {
	jp.FileName = filePath
	jp.RawSource = fileContent
	source.AddFile(jp.FileName)

	// 1. Define logical package (often based on root/subdir)
	jp.Package = jp.inferPackage(filePath)

	// 2. Preprocess source: Strip TS annotations, capture interfaces
	jp.CleanSource = frontend.Preprocess(jp.RawSource, jp.FileName, source)

	// 3. Trigger tree-sitter parse on cleaned output
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language()))

	tree := parser.Parse(jp.CleanSource, nil)
	defer tree.Close()

	root := tree.RootNode()
	jp.parseNode(root, "", source)
	return nil
}

func (jp *JavascriptParser) inferPackage(path string) string {
	dir := filepath.Dir(path)
	parts := strings.Split(filepath.ToSlash(dir), "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if last == "." || last == "" {
			return "frontend"
		}
		return last
	}
	return "frontend"
}

// parseNode traverses the resulting AST
func (jp *JavascriptParser) parseNode(node *tree_sitter.Node, parent string, source *store.Source) {
	if node == nil {
		return
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(uint(i))
		if child == nil {
			continue
		}

		kind := child.Kind()
		switch kind {
		case "class_declaration":
			jp.handleClass(child, source)
		case "function_declaration":
			jp.handleFunction(child, source)
		case "export_statement":
			// Dive in to look for declarations
			jp.parseNode(child, parent, source)
		case "lexical_declaration":
			// Matches 'const X = ...' (common for React functional components)
			jp.handleLexicalDeclaration(child, source)
		default:
			// Traverse children to not miss nested definitions
			if kind != "statement_block" && kind != "jsx_element" {
				jp.parseNode(child, parent, source)
			}
		}
	}
}

func (jp *JavascriptParser) handleClass(node *tree_sitter.Node, source *store.Source) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := NodeSource(jp.CleanSource, nameNode)

	lineNum := int(node.StartPosition().Row + 1)
	
	source.AddSymbol(store.Symbol{
		Name:    name,
		Kind:    store.SymStruct,
		File:    jp.FileName,
		Line:    lineNum,
		Package: jp.Package,
	})

	// Scan for methods
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		for j := 0; j < int(bodyNode.ChildCount()); j++ {
			meth := bodyNode.Child(uint(j))
			if meth != nil && meth.Kind() == "method_definition" {
				propNode := meth.ChildByFieldName("name")
				if propNode != nil {
					methodName := NodeSource(jp.CleanSource, propNode)
					source.AddSymbol(store.Symbol{
						Name:    methodName,
						Kind:    store.SymMethod,
						Parent:  name,
						File:    jp.FileName,
						Line:    int(meth.StartPosition().Row + 1),
						Package: jp.Package,
					})
					// Scan method body for nested components
					methText := NodeSource(jp.CleanSource, meth)
					frontend.ExtractJSXCalls(name, methText, source)
				}
			}
		}
	}
}

func (jp *JavascriptParser) handleFunction(node *tree_sitter.Node, source *store.Source) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := NodeSource(jp.CleanSource, nameNode)
	fullText := NodeSource(jp.CleanSource, node)

	lineNum := int(node.StartPosition().Row + 1)

	if frontend.IsReactComponent(name, fullText) {
		// Elevate functional component to a high-level struct/class representation
		frontend.RegisterComponent(name, jp.FileName, lineNum, source)
		frontend.ExtractJSXCalls(name, fullText, source)
	} else {
		source.AddSymbol(store.Symbol{
			Name:    name,
			Kind:    store.SymFunction,
			File:    jp.FileName,
			Line:    lineNum,
			Package: jp.Package,
		})
		// Still scan for possible JSX rendered from normal function helper
		frontend.ExtractJSXCalls(name, fullText, source)
	}
}

func (jp *JavascriptParser) handleLexicalDeclaration(node *tree_sitter.Node, source *store.Source) {
	// Look for declarator
	for i := 0; i < int(node.ChildCount()); i++ {
		decl := node.Child(uint(i))
		if decl == nil || decl.Kind() != "variable_declarator" {
			continue
		}
		nameNode := decl.ChildByFieldName("name")
		valueNode := decl.ChildByFieldName("value")
		if nameNode == nil || valueNode == nil {
			continue
		}
		
		name := NodeSource(jp.CleanSource, nameNode)
		fullValue := NodeSource(jp.CleanSource, valueNode)

		lineNum := int(decl.StartPosition().Row + 1)

		// If value is a function definition and passes component heuristic
		if (valueNode.Kind() == "arrow_function" || valueNode.Kind() == "function_expression") && 
		   frontend.IsReactComponent(name, fullValue) {
			
			source.AddSymbol(store.Symbol{
				Name:    name,
				Kind:    store.SymStruct, // Upgrade to Struct for component visibility
				File:    jp.FileName,
				Line:    lineNum,
				Package: jp.Package,
				Doc:     "React Functional Component",
			})
			frontend.ExtractJSXCalls(name, fullValue, source)
		} else if valueNode.Kind() == "arrow_function" || valueNode.Kind() == "function_expression" {
			// Normal top-level function export
			source.AddSymbol(store.Symbol{
				Name:    name,
				Kind:    store.SymFunction,
				File:    jp.FileName,
				Line:    lineNum,
				Package: jp.Package,
			})
			// Scan for component references inside normal exported arrows
			frontend.ExtractJSXCalls(name, fullValue, source)
		}
	}
}
