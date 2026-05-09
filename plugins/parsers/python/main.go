package main

import (
	"doc_generator/pkg/store"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// Parser is the exported parser implementation
var Parser store.Parser = &PythonParser{}

// Extensions is the list of file extensions this parser handles
var Extensions = []string{".py"}

// NodeSource extracts the raw source string corresponding to the start and end byte of a Tree-Sitter Node.
func NodeSource(source []byte, node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}

// PythonParser implements the store.Parser interface to extract declarations from Python files.
type PythonParser struct {
	FileName string
	File     []byte
}

// Parse extracts all classes, functions, and methods from the Python file into the source store.
func (pp *PythonParser) Parse(filePath string, fileContent []byte, source *store.Source) error {
	pp.FileName = filePath
	pp.File = fileContent
	source.AddFile(pp.FileName)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_python.Language()))

	tree := parser.Parse(pp.File, nil)
	defer tree.Close()

	root := tree.RootNode()

	pp.parseNode(root, "", source)
	return nil
}

// parseNode is a recursive helper that traverses the AST, dispatching recognized declarations.
func (pp *PythonParser) parseNode(node *tree_sitter.Node, currentClass string, source *store.Source) {
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
		case "class_definition":
			nextClass = pp.handleClass(child, source)
		case "function_definition":
			pp.handleFunction(child, currentClass, source)
		}

		pp.parseNode(child, nextClass, source)
	}
}

// getLeadingDocstring finds python triple-quoted string at the beginning of classes or functions.
func (pp *PythonParser) getDocstring(node *tree_sitter.Node) string {
	// Look inside class_definition or function_definition body (usually block node)
	body := node.ChildByFieldName("body")
	if body == nil {
		// Try finding first block or expression_statement child
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(uint(i))
			if c != nil && c.Kind() == "block" {
				body = c
				break
			}
		}
	}
	if body == nil {
		return ""
	}

	// First statement in block can be expression_statement containing string
	if body.ChildCount() > 0 {
		firstStmt := body.Child(0)
		if firstStmt != nil && firstStmt.Kind() == "expression_statement" {
			strNode := firstStmt.Child(0)
			if strNode != nil && strNode.Kind() == "string" {
				raw := NodeSource(pp.File, strNode)
				// Clean triple quotes
				raw = strings.TrimPrefix(raw, `"""`)
				raw = strings.TrimPrefix(raw, `'''`)
				raw = strings.TrimSuffix(raw, `"""`)
				raw = strings.TrimSuffix(raw, `'''`)
				return strings.TrimSpace(raw)
			}
		}
	}
	return ""
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

// findCalls recursively walks the children of a node to extract all call targets.
func (pp *PythonParser) findCalls(node *tree_sitter.Node, callerName string, source *store.Source) {
	if node == nil {
		return
	}

	if node.Kind() == "call" {
		fnNode := node.ChildByFieldName("function")
		if fnNode != nil {
			if fnNode.Kind() == "identifier" {
				callee := NodeSource(pp.File, fnNode)
				source.AddCall(callerName, callee)
			} else if fnNode.Kind() == "attribute" {
				attrNode := fnNode.ChildByFieldName("attribute")
				objNode := fnNode.ChildByFieldName("object")
				if attrNode != nil {
					methodName := NodeSource(pp.File, attrNode)
					if objNode != nil {
						objName := NodeSource(pp.File, objNode)
						source.AddCall(callerName, objName+"."+methodName)
					} else {
						source.AddCall(callerName, methodName)
					}
				}
			}
		}
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		pp.findCalls(node.Child(uint(i)), callerName, source)
	}
}

// handleClass extracts and registers python classes.
func (pp *PythonParser) handleClass(node *tree_sitter.Node, source *store.Source) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return ""
	}

	name := NodeSource(pp.File, nameNode)
	doc := pp.getDocstring(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)

	source.AddSymbol(store.Symbol{
		Name:          name,
		Kind:          store.SymStruct,
		File:          pp.FileName,
		Line:          int(node.StartPosition().Row + 1),
		Doc:           cleanedDoc,
		Audience:      aud,
		Compatibility: comp,
	})

	return name
}

// handleFunction extracts and registers python functions and receiver methods.
func (pp *PythonParser) handleFunction(node *tree_sitter.Node, parentClass string, source *store.Source) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}

	name := NodeSource(pp.File, nameNode)
	doc := pp.getDocstring(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)

	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = NodeSource(pp.File, paramsNode)
	}

	complexity := getComplexity(node) + 1
	startRow := node.StartPosition().Row
	endRow := node.EndPosition().Row
	lineCount := int(endRow - startRow + 1)

	kind := store.SymFunction
	if parentClass != "" {
		kind = store.SymMethod
	}

	source.AddSymbol(store.Symbol{
		Name:          name,
		Kind:          kind,
		File:          pp.FileName,
		Line:          int(node.StartPosition().Row + 1),
		Doc:           cleanedDoc,
		Audience:      aud,
		Compatibility: comp,
		Parent:        parentClass,
		Params:        params,
		LineCount:     lineCount,
		Complexity:    complexity,
	})

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		caller := name
		if parentClass != "" {
			caller = parentClass + "." + name
		}
		pp.findCalls(bodyNode, caller, source)
	}
}

// getComplexity recursively walks the AST of a function/method to estimate branches (if, for, while, except).
func getComplexity(node *tree_sitter.Node) int {
	if node == nil {
		return 0
	}
	complexity := 0
	kind := node.Kind()
	if kind == "if_statement" || kind == "for_statement" || kind == "while_statement" || kind == "except_clause" {
		complexity++
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		complexity += getComplexity(node.Child(uint(i)))
	}
	return complexity
}
