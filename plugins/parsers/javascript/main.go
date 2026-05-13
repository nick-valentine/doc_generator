package main

import (
	"doc_generator/pkg/parsers/frontend"
	anafrontend "doc_generator/pkg/analysis/frontend"
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

func isAsyncNode(node *tree_sitter.Node) bool {
	if node == nil {
		return false
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(uint(i)).Kind() == "async" {
			return true
		}
	}
	return false
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
	
	var relations []string
	// Find class_heritage which contains the base class
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		if child != nil && child.Kind() == "class_heritage" {
			// Class heritage can be 'extends Base'
			// We just take the entire text following 'extends' or the node value
			for k := 0; k < int(child.ChildCount()); k++ {
				cChild := child.Child(uint(k))
				if cChild != nil && cChild.Kind() != "extends" {
					relations = append(relations, NodeSource(jp.CleanSource, cChild))
				}
			}
		}
	}

	doc := jp.getLeadingComment(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)

	source.AddSymbol(store.Symbol{
		Name:          name,
		Kind:          store.SymStruct,
		File:          jp.FileName,
		Line:          lineNum,
		Package:       jp.Package,
		Doc:           cleanedDoc,
		Audience:      aud,
		Compatibility: comp,
		Relations:     relations,
		MemorySize:    jp.calculateShallowSize(node),
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
					methDoc := jp.getLeadingComment(meth)
					cDoc, mA, mC := parseAndCleanTags(methDoc)

					complexity := jp.getComplexity(meth) + 1
					lineCount := int(meth.EndPosition().Row - meth.StartPosition().Row + 1)
					paramsNode := meth.ChildByFieldName("parameters")
					params := ""
					if paramsNode != nil {
						params = NodeSource(jp.CleanSource, paramsNode)
					}

					spawnsThread := false
					bodyNode := meth.ChildByFieldName("body")
					if bodyNode != nil {
						spawnsThread = jp.hasThreadCreation(bodyNode)
					}

					source.AddSymbol(store.Symbol{
						Name:          methodName,
						Kind:          store.SymMethod,
						Parent:        name,
						File:          jp.FileName,
						Line:          int(meth.StartPosition().Row + 1),
						Package:       jp.Package,
						Doc:           cDoc,
						Audience:      mA,
						Compatibility: mC,
						LineCount:     lineCount,
						Complexity:    complexity,
						Params:        params,
						IsAsync:       isAsyncNode(meth),
						SpawnsThread:  spawnsThread,
					})

					caller := name + "." + methodName
					jp.findCalls(meth, caller, source)

					// Scan method body for nested components
					methText := NodeSource(jp.CleanSource, meth)
					anafrontend.ExtractJSXCalls(caller, methText, source)
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
	doc := jp.getLeadingComment(node)
	cleanedDoc, aud, comp := parseAndCleanTags(doc)
	complexity := jp.getComplexity(node) + 1
	lineCount := int(node.EndPosition().Row - node.StartPosition().Row + 1)
	paramsNode := node.ChildByFieldName("parameters")
	params := ""
	if paramsNode != nil {
		params = NodeSource(jp.CleanSource, paramsNode)
	}

	if anafrontend.IsReactComponent(name, fullText) {
		// Elevate functional component to a high-level struct/class representation
		anafrontend.RegisterComponent(name, jp.FileName, lineNum, source)
		// Update with complexity data directly
		for i := range source.Symbols {
			if source.Symbols[i].Name == name && source.Symbols[i].File == jp.FileName {
				source.Symbols[i].LineCount = lineCount
				source.Symbols[i].Complexity = complexity
				source.Symbols[i].Doc = cleanedDoc
				source.Symbols[i].Audience = aud
				source.Symbols[i].Params = params
				source.Symbols[i].IsAsync = isAsyncNode(node)
				break
			}
		}
		jp.findCalls(node, name, source)
		anafrontend.ExtractJSXCalls(name, fullText, source)
	} else {
		spawnsThread := false
		bodyNode := node.ChildByFieldName("body")
		if bodyNode != nil {
			spawnsThread = jp.hasThreadCreation(bodyNode)
		}

		source.AddSymbol(store.Symbol{
			Name:          name,
			Kind:          store.SymFunction,
			File:          jp.FileName,
			Line:          lineNum,
			Package:       jp.Package,
			Doc:           cleanedDoc,
			Audience:      aud,
			Compatibility: comp,
			Complexity:    complexity,
			LineCount:     lineCount,
			Params:        params,
			IsAsync:       isAsyncNode(node),
			SpawnsThread:  spawnsThread,
		})
		jp.findCalls(node, name, source)
		// Still scan for possible JSX rendered from normal function helper
		anafrontend.ExtractJSXCalls(name, fullText, source)
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
		doc := jp.getLeadingComment(node)
		cleanedDoc, aud, comp := parseAndCleanTags(doc)
		complexity := jp.getComplexity(valueNode) + 1
		lineCount := int(valueNode.EndPosition().Row - valueNode.StartPosition().Row + 1)
		
		paramsNode := valueNode.ChildByFieldName("parameters")
		params := ""
		if paramsNode != nil {
			params = NodeSource(jp.CleanSource, paramsNode)
		}

		spawnsThread := false
		bodyNode := valueNode.ChildByFieldName("body")
		if bodyNode != nil {
			spawnsThread = jp.hasThreadCreation(bodyNode)
		}

		// If value is a function definition and passes component heuristic
		if (valueNode.Kind() == "arrow_function" || valueNode.Kind() == "function_expression") && 
		   anafrontend.IsReactComponent(name, fullValue) {
			
			source.AddSymbol(store.Symbol{
				Name:          name,
				Kind:          store.SymStruct, // Upgrade to Struct for component visibility
				File:          jp.FileName,
				Line:          lineNum,
				Package:       jp.Package,
				Doc:           cleanedDoc,
				Audience:      aud,
				Compatibility: comp,
				Complexity:    complexity,
				LineCount:     lineCount,
				Params:        params,
				IsAsync:       isAsyncNode(valueNode),
				SpawnsThread:  spawnsThread,
			})
			jp.findCalls(valueNode, name, source)
			anafrontend.ExtractJSXCalls(name, fullValue, source)
		} else if valueNode.Kind() == "arrow_function" || valueNode.Kind() == "function_expression" {
			// Normal top-level function export
			source.AddSymbol(store.Symbol{
				Name:          name,
				Kind:          store.SymFunction,
				File:          jp.FileName,
				Line:          lineNum,
				Package:       jp.Package,
				Doc:           cleanedDoc,
				Audience:      aud,
				Compatibility: comp,
				Complexity:    complexity,
				LineCount:     lineCount,
				Params:        params,
				IsAsync:       isAsyncNode(valueNode),
				SpawnsThread:  spawnsThread,
			})
			jp.findCalls(valueNode, name, source)
			// Scan for component references inside normal exported arrows
			anafrontend.ExtractJSXCalls(name, fullValue, source)
		}
	}
}

func (jp *JavascriptParser) getLeadingComment(node *tree_sitter.Node) string {
	var comments []string
	prev := node.PrevSibling()
	for prev != nil {
		kind := prev.Kind()
		if kind == "comment" {
			txt := strings.TrimSpace(NodeSource(jp.CleanSource, prev))
			txt = strings.TrimPrefix(txt, "//")
			txt = strings.TrimPrefix(txt, "/*")
			txt = strings.TrimSuffix(txt, "*/")
			lines := strings.Split(txt, "\n")
			for k, l := range lines {
				lines[k] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "*"))
			}
			txt = strings.Join(lines, "\n")
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

func (jp *JavascriptParser) getComplexity(node *tree_sitter.Node) int {
	if node == nil {
		return 0
	}
	complexity := 0
	kind := node.Kind()
	if kind == "if_statement" || kind == "for_statement" || kind == "while_statement" || kind == "do_statement" || kind == "switch_statement" || kind == "conditional_expression" || kind == "catch_clause" {
		complexity++
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		complexity += jp.getComplexity(node.Child(uint(i)))
	}
	return complexity
}

func (jp *JavascriptParser) findCalls(node *tree_sitter.Node, callerName string, source *store.Source) {
	if node == nil {
		return
	}

	if node.Kind() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			if funcNode.Kind() == "identifier" {
				callee := NodeSource(jp.CleanSource, funcNode)
				source.AddCall(callerName, callee)
			} else if funcNode.Kind() == "member_expression" {
				propNode := funcNode.ChildByFieldName("property")
				objNode := funcNode.ChildByFieldName("object")
				if propNode != nil {
					methodName := NodeSource(jp.CleanSource, propNode)
					if objNode != nil {
						objName := NodeSource(jp.CleanSource, objNode)
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
		jp.findCalls(node.Child(uint(i)), callerName, source)
	}
}

func (jp *JavascriptParser) hasThreadCreation(node *tree_sitter.Node) bool {
	if node == nil { return false }
	kind := node.Kind()

	if kind == "new_expression" {
		cons := node.ChildByFieldName("constructor")
		if cons != nil && strings.Contains(NodeSource(jp.CleanSource, cons), "Worker") {
			return true
		}
	} else if kind == "call_expression" {
		fnNode := node.ChildByFieldName("function")
		if fnNode != nil {
			txt := NodeSource(jp.CleanSource, fnNode)
			if strings.Contains(txt, "fork") || strings.Contains(txt, "Worker") || strings.Contains(txt, "spawn") {
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

func (jp *JavascriptParser) calculateShallowSize(node *tree_sitter.Node) int {
	fields := make(map[string]bool)
	bodyNode := node.ChildByFieldName("body")
	if bodyNode == nil {
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(uint(i))
			if c != nil && c.Kind() == "class_body" {
				bodyNode = c
				break
			}
		}
	}
	if bodyNode == nil { return 0 }

	for i := 0; i < int(bodyNode.ChildCount()); i++ {
		c := bodyNode.Child(uint(i))
		if c == nil { continue }
		k := c.Kind()
		if k == "public_field_definition" {
			pName := c.ChildByFieldName("name")
			if pName != nil {
				fields[NodeSource(jp.CleanSource, pName)] = true
			}
		} else if k == "method_definition" {
			// Check if constructor
			mNameNode := c.ChildByFieldName("name")
			if mNameNode != nil && NodeSource(jp.CleanSource, mNameNode) == "constructor" {
				jp.extractConstructorFields(c, fields)
			}
		}
	}

	return len(fields) * 8
}

func (jp *JavascriptParser) extractConstructorFields(node *tree_sitter.Node, fields map[string]bool) {
	if node == nil { return }
	if node.Kind() == "assignment_expression" {
		left := node.ChildByFieldName("left")
		if left != nil && left.Kind() == "member_expression" {
			obj := left.ChildByFieldName("object")
			if obj != nil && NodeSource(jp.CleanSource, obj) == "this" {
				prop := left.ChildByFieldName("property")
				if prop != nil {
					fields[NodeSource(jp.CleanSource, prop)] = true
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		jp.extractConstructorFields(node.Child(uint(i)), fields)
	}
}
