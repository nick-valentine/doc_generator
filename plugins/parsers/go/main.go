package main

import (
	"doc_generator/pkg/store"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// Parser is the exported parser implementation
var Parser store.Parser = &GoParser{}

// Extensions is the list of file extensions this parser handles
var Extensions = []string{".go"}

// NodeSource extracts the raw source string corresponding to the start and end byte of a Tree-Sitter Node.
func NodeSource(source []byte, node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}

// GoParser implements the store.Parser interface to extract declarations from Go files.
type GoParser struct {
	// FileName is the file path of the Go file.
	FileName string
	// File contains the raw byte contents of the Go file.
	File []byte
	// Package is the package name.
	Package string
}

// Parse extracts all functions, structures, receiver methods, and fields from the Go file into the source store.
func (gp *GoParser) Parse(filePath string, fileContent []byte, source *store.Source) error {
	gp.FileName = filePath
	gp.File = fileContent
	source.AddFile(gp.FileName)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_go.Language()))

	tree := parser.Parse(gp.File, nil)
	defer tree.Close()

	root := tree.RootNode()

	// Find package name
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(uint(i))
		if child != nil && child.Kind() == "package_clause" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				gp.Package = NodeSource(gp.File, nameNode)
				source.AddSymbol(store.Symbol{
					Name:    gp.Package,
					Kind:    "package",
					File:    gp.FileName,
					Line:    int(child.StartPosition().Row + 1),
					Package: gp.Package,
				})
			}
			break
		}
	}

	gp.parseNode(root, source)
	return nil
}

// parseNode is a recursive helper that traverses the AST, dispatching recognized declarations.
func (gp *GoParser) parseNode(node *tree_sitter.Node, source *store.Source) {
	if node == nil {
		return
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(uint(i))
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "function_declaration":
			if child.Parent() != nil && child.Parent().Kind() == "source_file" {
				gp.handleFunction(child, source)
			}
		case "method_declaration":
			if child.Parent() != nil && child.Parent().Kind() == "source_file" {
				gp.handleMethod(child, source)
			}
		case "type_declaration":
			if child.Parent() != nil && child.Parent().Kind() == "source_file" {
				gp.handleTypeDeclaration(child, source)
			}
		case "import_declaration":
			if child.Parent() != nil && child.Parent().Kind() == "source_file" {
				gp.handleImport(child, source)
			}
		case "var_declaration", "const_declaration":
			if child.Parent() != nil && child.Parent().Kind() == "source_file" {
				gp.handleVariable(child, source)
			}
		case "comment":
			gp.handleComment(child, source)
		}

		gp.parseNode(child, source)
	}
}

// getLeadingComment finds all contiguous comment nodes directly preceding the given node.
func (gp *GoParser) getLeadingComment(node *tree_sitter.Node) string {
	var comments []string
	prev := node.PrevSibling()
	for prev != nil {
		kind := prev.Kind()
		if kind == "comment" {
			txt := strings.TrimSpace(NodeSource(gp.File, prev))
			// Strip comment markers
			txt = strings.TrimPrefix(txt, "//")
			txt = strings.TrimPrefix(txt, "/*")
			txt = strings.TrimSuffix(txt, "*/")
			txt = strings.TrimSpace(txt)
			comments = append([]string{txt}, comments...)
			prev = prev.PrevSibling()
		} else if kind == "escape_sequence" || prev.StartByte() == node.StartByte() {
			// Skip compiler/AST artifacts if any
			prev = prev.PrevSibling()
		} else {
			break
		}
	}
	return strings.Join(comments, "\n")
}

// parseAndCleanTags parses @audience and @compatibility tags, stripping them from the raw doc comment.
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

// findCalls recursively walks the children of a node to extract all call_expression targets inside a function or method body.
func (gp *GoParser) findCalls(node *tree_sitter.Node, callerName string, source *store.Source) {
	if node == nil {
		return
	}

	if node.Kind() == "call_expression" {
		fnNode := node.ChildByFieldName("function")
		if fnNode != nil {
			if fnNode.Kind() == "identifier" {
				callee := NodeSource(gp.File, fnNode)
				source.AddCall(callerName, callee)
			} else if fnNode.Kind() == "selector_expression" {
				fieldNode := fnNode.ChildByFieldName("field")
				operandNode := fnNode.ChildByFieldName("operand")
				if fieldNode != nil {
					methodName := NodeSource(gp.File, fieldNode)
					if operandNode != nil {
						operandName := NodeSource(gp.File, operandNode)
						source.AddCall(callerName, operandName+"."+methodName)
					} else {
						source.AddCall(callerName, methodName)
					}
				}
			}
		}
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		gp.findCalls(node.Child(uint(i)), callerName, source)
	}
}

// handleFunction extracts and registers a global function symbol from the AST.
func (gp *GoParser) handleFunction(node *tree_sitter.Node, source *store.Source) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := NodeSource(gp.File, nameNode)
	doc := gp.getLeadingComment(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)

	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = NodeSource(gp.File, paramsNode)
	}

	resultNode := node.ChildByFieldName("result")
	var returns string
	if resultNode != nil {
		returns = NodeSource(gp.File, resultNode)
	}

	complexity := getComplexity(node) + 1
	startRow := node.StartPosition().Row
	endRow := node.EndPosition().Row
	lineCount := int(endRow - startRow + 1)

	source.AddSymbol(store.Symbol{
		Name:          name,
		Kind:          store.SymFunction,
		File:          gp.FileName,
		Line:          int(node.StartPosition().Row + 1),
		Doc:           cleanedDoc,
		Audience:      aud,
		Compatibility: comp,
		Package:       gp.Package,
		Params:        params,
		Returns:       returns,
		LineCount:     lineCount,
		Complexity:    complexity,
	})

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		caller := name
		if gp.Package != "" {
			caller = gp.Package + "." + name
		}
		gp.findCalls(bodyNode, caller, source)
	}
}

// handleMethod extracts and registers a struct method receiver symbol from the AST.
func (gp *GoParser) handleMethod(node *tree_sitter.Node, source *store.Source) {
	nameNode := node.ChildByFieldName("name")
	recNode := node.ChildByFieldName("receiver")
	if nameNode == nil || recNode == nil {
		return
	}
	name := NodeSource(gp.File, nameNode)

	// Receiver can be "(r *Config)" or "(r Config)". We want to extract "Config" as parent.
	recStr := NodeSource(gp.File, recNode)
	parent := extractReceiverType(recStr)

	doc := gp.getLeadingComment(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)

	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = NodeSource(gp.File, paramsNode)
	}

	resultNode := node.ChildByFieldName("result")
	var returns string
	if resultNode != nil {
		returns = NodeSource(gp.File, resultNode)
	}

	complexity := getComplexity(node) + 1
	startRow := node.StartPosition().Row
	endRow := node.EndPosition().Row
	lineCount := int(endRow - startRow + 1)

	source.AddSymbol(store.Symbol{
		Name:          name,
		Kind:          store.SymMethod,
		File:          gp.FileName,
		Line:          int(node.StartPosition().Row + 1),
		Doc:           cleanedDoc,
		Audience:      aud,
		Compatibility: comp,
		Package:       gp.Package,
		Parent:        parent,
		Params:        params,
		Returns:       returns,
		LineCount:     lineCount,
		Complexity:    complexity,
	})

	callerName := name
	if parent != "" {
		callerName = parent + "." + name
	}
	if gp.Package != "" {
		callerName = gp.Package + "." + callerName
	}

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		gp.findCalls(bodyNode, callerName, source)
	}
}

// handleTypeDeclaration extracts and registers structured types (like structs and their fields) from the AST.
func (gp *GoParser) handleTypeDeclaration(node *tree_sitter.Node, source *store.Source) {
	// type_declaration contains type_spec
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(uint(i))
		if child != nil && child.Kind() == "type_spec" {
			nameNode := child.ChildByFieldName("name")
			typeNode := child.ChildByFieldName("type")
			if nameNode == nil || typeNode == nil {
				continue
			}

			name := NodeSource(gp.File, nameNode)
			doc := gp.getLeadingComment(node) // leading comment is usually on the type_declaration node
			cleanedDoc, aud, comp := parseAndCleanTags(doc)

			if typeNode.Kind() == "struct_type" {
				source.AddSymbol(store.Symbol{
					Name:          name,
					Kind:          store.SymStruct,
					File:          gp.FileName,
					Line:          int(node.StartPosition().Row + 1),
					Doc:           cleanedDoc,
					Audience:      aud,
					Compatibility: comp,
					Package:       gp.Package,
				})

				gp.handleStructFields(typeNode, name, source)
			} else if typeNode.Kind() == "interface_type" {
				source.AddSymbol(store.Symbol{
					Name:          name,
					Kind:          store.SymInterface,
					File:          gp.FileName,
					Line:          int(node.StartPosition().Row + 1),
					Doc:           cleanedDoc,
					Audience:      aud,
					Compatibility: comp,
					Package:       gp.Package,
				})
			}
		}
	}
}

// handleStructFields extracts and registers individual fields belonging to a parent struct.
func (gp *GoParser) handleStructFields(structNode *tree_sitter.Node, structName string, source *store.Source) {
	// Inside struct_type we have field_declaration_list
	fieldsList := structNode.ChildByFieldName("fields")
	if fieldsList == nil {
		// Look for first child list
		for i := 0; i < int(structNode.ChildCount()); i++ {
			c := structNode.Child(uint(i))
			if c != nil && c.Kind() == "field_declaration_list" {
				fieldsList = c
				break
			}
		}
	}
	if fieldsList == nil {
		return
	}

	count := int(fieldsList.ChildCount())
	for i := 0; i < count; i++ {
		fieldDecl := fieldsList.Child(uint(i))
		if fieldDecl != nil && fieldDecl.Kind() == "field_declaration" {
			// field_declaration can have multiple names: "A, B int"
			nameNode := fieldDecl.ChildByFieldName("name")
			var names []string
			if nameNode != nil {
				names = append(names, NodeSource(gp.File, nameNode))
			} else {
				// Find all identifier children that represent names
				for j := 0; j < int(fieldDecl.ChildCount()); j++ {
					c := fieldDecl.Child(uint(j))
					if c != nil && (c.Kind() == "field_identifier" || c.Kind() == "identifier") {
						names = append(names, NodeSource(gp.File, c))
					}
				}
			}

			doc := gp.getLeadingComment(fieldDecl)
			cleanedDoc, aud, comp := parseAndCleanTags(doc)

			typeNode := fieldDecl.ChildByFieldName("type")
			var typeStr string
			if typeNode != nil {
				typeStr = NodeSource(gp.File, typeNode)
			}

			for _, fName := range names {
				source.AddSymbol(store.Symbol{
					Name:          fName,
					Kind:          store.SymField,
					File:          gp.FileName,
					Line:          int(fieldDecl.StartPosition().Row + 1),
					Doc:           cleanedDoc,
					Audience:      aud,
					Compatibility: comp,
					Parent:        structName,
					Package:       gp.Package,
					Type:          typeStr,
				})
			}
		}
	}
}

// extractReceiverType extracts the clean type identifier from receiver declaration strings.
func extractReceiverType(recStr string) string {
	// recStr looks like "(r *Config)" or "(Config)"
	recStr = strings.TrimPrefix(recStr, "(")
	recStr = strings.TrimSuffix(recStr, ")")
	recStr = strings.TrimSpace(recStr)

	parts := strings.Fields(recStr)
	var typePart string
	if len(parts) > 1 {
		typePart = parts[1]
	} else if len(parts) == 1 {
		typePart = parts[0]
	}

	typePart = strings.TrimPrefix(typePart, "*")
	return typePart
}

// handleImport extracts and registers Go import statements.
func (gp *GoParser) handleImport(node *tree_sitter.Node, source *store.Source) {
	var walk func(n *tree_sitter.Node)
	walk = func(n *tree_sitter.Node) {
		if n == nil {
			return
		}
		if n.Kind() == "import_spec" {
			pathNode := n.ChildByFieldName("path")
			if pathNode != nil {
				importPath := strings.Trim(NodeSource(gp.File, pathNode), `"`)
				source.AddSymbol(store.Symbol{
					Name: importPath,
					Kind: store.SymImport,
					File: gp.FileName,
					Line: int(n.StartPosition().Row + 1),
				})
			}
			return
		}
		count := int(n.ChildCount())
		for i := 0; i < count; i++ {
			walk(n.Child(uint(i)))
		}
	}
	walk(node)
}

// handleVariable extracts global/package level variables and constants.
func (gp *GoParser) handleVariable(node *tree_sitter.Node, source *store.Source) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(uint(i))
		if child != nil && (child.Kind() == "var_spec" || child.Kind() == "const_spec") {
			nameNode := child.ChildByFieldName("name")
			var names []string
			if nameNode != nil {
				names = append(names, NodeSource(gp.File, nameNode))
			} else {
				for j := 0; j < int(child.ChildCount()); j++ {
					c := child.Child(uint(j))
					if c != nil && (c.Kind() == "identifier" || c.Kind() == "field_identifier") {
						names = append(names, NodeSource(gp.File, c))
					}
				}
			}
			typeNode := child.ChildByFieldName("type")
			var typeStr string
			if typeNode != nil {
				typeStr = NodeSource(gp.File, typeNode)
			}
			doc := gp.getLeadingComment(node)
			cleanedDoc, aud, comp := parseAndCleanTags(doc)

			for _, name := range names {
				source.AddSymbol(store.Symbol{
					Name:          name,
					Kind:          store.SymVariable,
					File:          gp.FileName,
					Line:          int(node.StartPosition().Row + 1),
					Doc:           cleanedDoc,
					Audience:      aud,
					Compatibility: comp,
					Type:          typeStr,
					Package:       gp.Package,
				})
			}
		}
	}
}

// handleComment checks comments for TODO tokens and registers them.
func (gp *GoParser) handleComment(node *tree_sitter.Node, source *store.Source) {
	txt := NodeSource(gp.File, node)
	if strings.Contains(txt, "TODO") {
		cleanTxt := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(txt, "//"), "/*"))
		source.AddSymbol(store.Symbol{
			Name: "TODO",
			Kind: store.SymVariable,
			File: gp.FileName,
			Line: int(node.StartPosition().Row + 1),
			Doc:  cleanTxt,
		})
	}
}

// getComplexity recursively walks the AST of a function/method to estimate branches (if, for, case).
func getComplexity(node *tree_sitter.Node) int {
	if node == nil {
		return 0
	}
	complexity := 0
	kind := node.Kind()
	if kind == "if_statement" || kind == "for_statement" || kind == "expression_case_clause" || kind == "type_case_clause" {
		complexity++
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		complexity += getComplexity(node.Child(uint(i)))
	}
	return complexity
}
