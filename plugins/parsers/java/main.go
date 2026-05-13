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

func isAsyncJava(node *tree_sitter.Node, returns string, fileContent []byte) bool {
	if strings.Contains(returns, "Future") || strings.Contains(returns, "Mono") || strings.Contains(returns, "Flux") || strings.Contains(returns, "CompletionStage") {
		return true
	}
	txt := NodeSource(fileContent, node)
	if strings.Contains(txt, "@Async") {
		return true
	}
	return false
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

// findCalls recursively walks the children of a node to extract method calls, constructor invocations, and references.
func (jp *JavaParser) findCalls(node *tree_sitter.Node, callerName string, source *store.Source) {
	if node == nil {
		return
	}

	kind := node.Kind()

	switch kind {
	case "method_invocation":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			methodName := NodeSource(jp.File, nameNode)
			objectNode := node.ChildByFieldName("object")
			if objectNode != nil {
				objectName := NodeSource(jp.File, objectNode)
				source.AddCall(callerName, objectName+"."+methodName)
			} else {
				source.AddCall(callerName, methodName)
			}
		}
	case "object_creation_expression":
		// E.g., new MyObject(...)
		typeNode := node.ChildByFieldName("type")
		if typeNode != nil {
			typeName := NodeSource(jp.File, typeNode)
			// Record the call to the Type's constructor
			source.AddCall(callerName, typeName)
		}
	case "explicit_constructor_invocation":
		// E.g., super(...) or this(...)
		consNode := node.ChildByFieldName("constructor")
		if consNode != nil {
			consName := NodeSource(jp.File, consNode)
			source.AddCall(callerName, consName)
		}
	case "method_reference":
		// E.g., System.out::println or MyClass::staticMethod
		// Usually 3 children: LHS, '::', and the identifier RHS.
		// We look for an identifier child node that isn't the first child if we want reliable parsing,
		// or just iterate looking for specific elements.
		var targetMethod string
		cCount := int(node.ChildCount())
		for i := 0; i < cCount; i++ {
			child := node.Child(uint(i))
			if child != nil && child.Kind() == "identifier" {
				targetMethod = NodeSource(jp.File, child)
			}
		}
		// First child provides context
		if targetMethod != "" && cCount > 0 {
			contextNode := node.Child(0)
			if contextNode != nil {
				contextName := NodeSource(jp.File, contextNode)
				source.AddCall(callerName, contextName+"."+targetMethod)
			} else {
				source.AddCall(callerName, targetMethod)
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

	memSize := jp.calculateShallowSize(node)

	source.AddSymbol(store.Symbol{
		Name:          name,
		Kind:          kind,
		File:          jp.FileName,
		Line:          int(node.StartPosition().Row + 1),
		Doc:           cleanedDoc,
		Audience:      aud,
		Compatibility: comp,
		Package:       jp.Package,
		MemorySize:    memSize,
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

	spawnsThread := false
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		spawnsThread = jp.hasThreadCreation(bodyNode)
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
		IsAsync:       isAsyncJava(node, returns, jp.File),
		SpawnsThread:  spawnsThread,
	})

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

func calculateJavaTypeSize(t string) int {
	t = strings.TrimSpace(t)
	switch t {
	case "byte", "boolean":
		return 1
	case "short", "char":
		return 2
	case "int", "float":
		return 4
	case "long", "double":
		return 8
	}
	// References
	return 8
}

func (jp *JavaParser) hasThreadCreation(node *tree_sitter.Node) bool {
	if node == nil { return false }
	
	kind := node.Kind()
	if kind == "object_creation_expression" {
		typeNode := node.ChildByFieldName("type")
		if typeNode != nil {
			tName := NodeSource(jp.File, typeNode)
			if strings.Contains(tName, "Thread") || strings.Contains(tName, "Executor") {
				return true
			}
		}
	} else if kind == "method_invocation" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			mName := NodeSource(jp.File, nameNode)
			if mName == "start" || mName == "execute" || mName == "submit" || mName == "runAsync" {
				return true
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		if jp.hasThreadCreation(node.Child(uint(i))) {
			return true
		}
	}
	return false
}

func (jp *JavaParser) calculateShallowSize(node *tree_sitter.Node) int {
	total := 0
	bodyNode := node.ChildByFieldName("body")
	if bodyNode == nil {
		// Find class_body child
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(uint(i))
			if c != nil && c.Kind() == "class_body" {
				bodyNode = c
				break
			}
		}
	}
	if bodyNode == nil {
		return 0
	}

	for i := 0; i < int(bodyNode.ChildCount()); i++ {
		c := bodyNode.Child(uint(i))
		if c != nil && c.Kind() == "field_declaration" {
			total += jp.sumFieldDeclarations(c)
		}
	}
	return total
}

func (jp *JavaParser) sumFieldDeclarations(node *tree_sitter.Node) int {
	typeNode := node.ChildByFieldName("type")
	typeStr := "Object" // default fallback
	if typeNode != nil {
		typeStr = NodeSource(jp.File, typeNode)
	}
	sz := calculateJavaTypeSize(typeStr)
	
	declaratorCount := 0
	var walkDeclarators func(n *tree_sitter.Node)
	walkDeclarators = func(n *tree_sitter.Node) {
		if n == nil { return }
		if n.Kind() == "variable_declarator" {
			declaratorCount++
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walkDeclarators(n.Child(uint(i)))
		}
	}
	walkDeclarators(node)
	
	if declaratorCount == 0 {
		declaratorCount = 1
	}
	return sz * declaratorCount
}
