package main

import (
	"doc_generator/pkg/store"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// Parser is the exported parser implementation
var Parser store.Parser = &JavaParser{}

// Extensions is the list of file extensions this parser handles
var Extensions = []string{".java"}

// NodeSource extracts the raw source string corresponding to the start and end byte of a Tree-Sitter Node.
func NodeSource(source []byte, node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}

// JavaParser implements the store.Parser interface to extract declarations from Java files.
type JavaParser struct {
	FileName string
	File     []byte
	Package  string
}

// Parse extracts all classes, interfaces, methods, and fields from the Java file into the source store.
func (jp *JavaParser) Parse(filePath string, fileContent []byte, source *store.Source) error {
	jp.FileName = filePath
	jp.File = fileContent
	source.AddFile(jp.FileName)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_java.Language()))

	tree := parser.Parse(jp.File, nil)
	defer tree.Close()

	root := tree.RootNode()

	// Find package name
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(uint(i))
		if child != nil && child.Kind() == "package_declaration" {
			// Find the scoped_identifier or identifier inside package_declaration
			for j := 0; j < int(child.ChildCount()); j++ {
				c := child.Child(uint(j))
				if c != nil && (c.Kind() == "scoped_identifier" || c.Kind() == "identifier") {
					jp.Package = NodeSource(jp.File, c)
					source.AddSymbol(store.Symbol{
						Name:    jp.Package,
						Kind:    "package",
						File:    jp.FileName,
						Line:    int(child.StartPosition().Row + 1),
						Package: jp.Package,
					})
					break
				}
			}
			break
		}
	}

	jp.parseNode(root, "", source)
	return nil
}

// parseNode is a recursive helper that traverses the AST, dispatching recognized declarations.
func (jp *JavaParser) parseNode(node *tree_sitter.Node, currentClass string, source *store.Source) {
	if node == nil {
		return
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(uint(i))
		if child == nil {
			continue
		}

		nextClass := currentClass
		switch child.Kind() {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
			nextClass = jp.handleClassOrInterface(child, source)
		case "method_declaration", "constructor_declaration":
			jp.handleMethod(child, currentClass, source)
		case "field_declaration":
			jp.handleField(child, currentClass, source)
		}

		jp.parseNode(child, nextClass, source)
	}
}

// getLeadingComment finds all contiguous comment nodes directly preceding the given node.
func (jp *JavaParser) getLeadingComment(node *tree_sitter.Node) string {
	var comments []string
	prev := node.PrevSibling()
	for prev != nil {
		kind := prev.Kind()
		if kind == "block_comment" || kind == "line_comment" {
			txt := strings.TrimSpace(NodeSource(jp.File, prev))
			txt = strings.TrimPrefix(txt, "//")
			txt = strings.TrimPrefix(txt, "/*")
			txt = strings.TrimSuffix(txt, "*/")
			// Strip additional * at line starts
			lines := strings.Split(txt, "\n")
			for k, l := range lines {
				lines[k] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "*"))
			}
			txt = strings.Join(lines, "\n")
			txt = strings.TrimSpace(txt)
			comments = append([]string{txt}, comments...)
			prev = prev.PrevSibling()
		} else if prev.StartByte() == node.StartByte() {
			prev = prev.PrevSibling()
		} else {
			break
		}
	}
	return strings.Join(comments, "\n")
}

func parseAndCleanTags(doc string) (cleanedDoc string, audience []string, compatibility []string) {
	lines := strings.Split(doc, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@audience") {
			parts := strings.Fields(trimmed)
			if len(parts) > 1 {
				audience = append(audience, parts[1:]...)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "@compatibility") {
			parts := strings.Fields(trimmed)
			if len(parts) > 1 {
				compatibility = append(compatibility, parts[1:]...)
			}
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}
	cleanedDoc = strings.TrimSpace(strings.Join(cleanedLines, "\n"))
	return cleanedDoc, audience, compatibility
}

// findCalls recursively walks the children of a node to extract all method_invocation targets.
func (jp *JavaParser) findCalls(node *tree_sitter.Node, callerName string, source *store.Source) {
	if node == nil {
		return
	}

	if node.Kind() == "method_invocation" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			methodName := NodeSource(jp.File, nameNode)
			// Check if there is an object calling it
			objectNode := node.ChildByFieldName("object")
			if objectNode != nil {
				objectName := NodeSource(jp.File, objectNode)
				source.AddCall(callerName, objectName+"."+methodName)
			} else {
				source.AddCall(callerName, methodName)
			}
		}
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		jp.findCalls(node.Child(uint(i)), callerName, source)
	}
}

// handleClassOrInterface extracts and registers classes and interfaces.
func (jp *JavaParser) handleClassOrInterface(node *tree_sitter.Node, source *store.Source) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		// Find first identifier as fallback
		for j := 0; j < int(node.ChildCount()); j++ {
			c := node.Child(uint(j))
			if c != nil && c.Kind() == "identifier" {
				nameNode = c
				break
			}
		}
	}
	if nameNode == nil {
		return ""
	}

	name := NodeSource(jp.File, nameNode)
	doc := jp.getLeadingComment(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)

	kind := store.SymStruct
	if node.Kind() == "interface_declaration" {
		kind = store.SymInterface
	}

	source.AddSymbol(store.Symbol{
		Name:          name,
		Kind:          kind,
		File:          jp.FileName,
		Line:          int(node.StartPosition().Row + 1),
		Doc:           cleanedDoc,
		Audience:      aud,
		Compatibility: comp,
		Package:       jp.Package,
	})

	return name
}

// handleMethod extracts and registers methods and constructors.
func (jp *JavaParser) handleMethod(node *tree_sitter.Node, parentClass string, source *store.Source) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		// Find first identifier as fallback
		for j := 0; j < int(node.ChildCount()); j++ {
			c := node.Child(uint(j))
			if c != nil && c.Kind() == "identifier" {
				nameNode = c
				break
			}
		}
	}
	if nameNode == nil {
		return
	}

	name := NodeSource(jp.File, nameNode)
	doc := jp.getLeadingComment(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)

	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = NodeSource(jp.File, paramsNode)
	}

	typeNode := node.ChildByFieldName("type")
	var returns string
	if typeNode != nil {
		returns = NodeSource(jp.File, typeNode)
	}

	complexity := getComplexity(node) + 1
	startRow := node.StartPosition().Row
	endRow := node.EndPosition().Row
	lineCount := int(endRow - startRow + 1)

	kind := store.SymMethod
	if node.Kind() == "constructor_declaration" {
		kind = store.SymMethod // Constructors are registered as methods
	}

	source.AddSymbol(store.Symbol{
		Name:          name,
		Kind:          kind,
		File:          jp.FileName,
		Line:          int(node.StartPosition().Row + 1),
		Doc:           cleanedDoc,
		Audience:      aud,
		Compatibility: comp,
		Package:       jp.Package,
		Parent:        parentClass,
		Params:        params,
		Returns:       returns,
		LineCount:     lineCount,
		Complexity:    complexity,
	})

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		caller := name
		if parentClass != "" {
			caller = parentClass + "." + name
		}
		if jp.Package != "" {
			caller = jp.Package + "." + caller
		}
		jp.findCalls(bodyNode, caller, source)
	}
}

// handleField extracts and registers fields.
func (jp *JavaParser) handleField(node *tree_sitter.Node, parentClass string, source *store.Source) {
	// field_declaration can have variable_declarator child nodes
	doc := jp.getLeadingComment(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)

	typeNode := node.ChildByFieldName("type")
	var typeStr string
	if typeNode != nil {
		typeStr = NodeSource(jp.File, typeNode)
	}

	var walkDeclarators func(n *tree_sitter.Node)
	walkDeclarators = func(n *tree_sitter.Node) {
		if n == nil {
			return
		}
		if n.Kind() == "variable_declarator" {
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				fName := NodeSource(jp.File, nameNode)
				source.AddSymbol(store.Symbol{
					Name:          fName,
					Kind:          store.SymField,
					File:          jp.FileName,
					Line:          int(node.StartPosition().Row + 1),
					Doc:           cleanedDoc,
					Audience:      aud,
					Compatibility: comp,
					Parent:        parentClass,
					Package:       jp.Package,
					Type:          typeStr,
				})
			}
			return
		}
		count := int(n.ChildCount())
		for i := 0; i < count; i++ {
			walkDeclarators(n.Child(uint(i)))
		}
	}
	walkDeclarators(node)
}

// getComplexity recursively walks the AST of a method to estimate branches (if, for, while, switch).
func getComplexity(node *tree_sitter.Node) int {
	if node == nil {
		return 0
	}
	complexity := 0
	kind := node.Kind()
	if kind == "if_statement" || kind == "for_statement" || kind == "while_statement" || kind == "switch_label" || kind == "catch_clause" {
		complexity++
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		complexity += getComplexity(node.Child(uint(i)))
	}
	return complexity
}
