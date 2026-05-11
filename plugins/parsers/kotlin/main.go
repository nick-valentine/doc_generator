package main

import (
	"doc_generator/pkg/store"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_kotlin "github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go"
)

// Parser is the exported parser implementation
var Parser store.Parser = &KotlinParser{}

// Extensions is the list of file extensions this parser handles
var Extensions = []string{".kt"}

// NodeSource extracts raw source string for node
func NodeSource(source []byte, node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}

type KotlinParser struct {
	FileName string
	File     []byte
	Package  string
}

func (kp *KotlinParser) Parse(filePath string, fileContent []byte, source *store.Source) error {
	kp.FileName = filePath
	kp.File = fileContent
	kp.Package = "main" // Default
	source.AddFile(kp.FileName)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_kotlin.Language()))

	tree := parser.Parse(kp.File, nil)
	defer tree.Close()

	root := tree.RootNode()

	// 1. Fast scan for package header at root
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(uint(i))
		if child != nil && child.Kind() == "package_header" {
			for j := 0; j < int(child.ChildCount()); j++ {
				pChild := child.Child(uint(j))
				if pChild != nil && pChild.Kind() == "qualified_identifier" {
					kp.Package = strings.TrimSpace(NodeSource(kp.File, pChild))
					break
				}
			}
			source.AddSymbol(store.Symbol{
				Name:    kp.Package,
				Kind:    "package",
				File:    kp.FileName,
				Line:    1,
				Package: kp.Package,
			})
			break
		}
	}

	// 2. Recursive traversal with strict scoping
	kp.parseNode(root, "", source)

	return nil
}

func (kp *KotlinParser) parseNode(node *tree_sitter.Node, parent string, source *store.Source) {
	if node == nil {
		return
	}

	kind := node.Kind()

	// Handle Class / Object / Interface
	if kind == "class_declaration" || kind == "interface_declaration" || kind == "object_declaration" || kind == "companion_object" {
		name := ""
		if kind == "companion_object" {
			name = "Companion"
		} else {
			// Find direct identifier
			for i := 0; i < int(node.ChildCount()); i++ {
				c := node.Child(uint(i))
				if c != nil && c.Kind() == "identifier" {
					name = NodeSource(kp.File, c)
					break
				}
			}
		}

		if name != "" {
			symKind := store.SymStruct
			if kind == "interface_declaration" {
				symKind = store.SymInterface
			}

			// User Request: Scope companion / nested objects to their owner's namespace.
			storedName := name
			if parent != "" {
				storedName = parent + "." + name
			}

			var relations []string
			// Look for delegation_specifiers (inheritance)
			for i := 0; i < int(node.ChildCount()); i++ {
				c := node.Child(uint(i))
				if c != nil && c.Kind() == "delegation_specifiers" {
					relations = kp.extractRelations(c)
					break
				}
			}

			doc := kp.getLeadingComment(node)
			cleanedDoc, aud, comp := parseAndCleanTags(doc)

			memSize := kp.calculateShallowSize(node)

			source.AddSymbol(store.Symbol{
				Name:          storedName,
				Kind:          symKind,
				Parent:        parent,
				File:          kp.FileName,
				Line:          int(node.StartPosition().Row + 1),
				Package:       kp.Package,
				Doc:           cleanedDoc,
				Audience:      aud,
				Compatibility: comp,
				Relations:     relations,
				MemorySize:    memSize,
			})

			// Traverse children with NEW context (scoped to this class) using storedName as the parent context
			for i := 0; i < int(node.ChildCount()); i++ {
				c := node.Child(uint(i))
				if c == nil { continue }
				if c.Kind() == "primary_constructor" {
					kp.parsePrimaryConstructor(c, storedName, source)
				} else if c.Kind() == "class_body" {
					// Traverse class body elements with fully qualified storedName
					for j := 0; j < int(c.ChildCount()); j++ {
						kp.parseNode(c.Child(uint(j)), storedName, source)
					}
				}
			}
			return // Handled recursively
		}
	}

	// Handle Function
	if kind == "function_declaration" {
		name := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(uint(i))
			if c != nil && c.Kind() == "identifier" {
				name = NodeSource(kp.File, c)
				break
			}
		}

		if name != "" {
			params := ""
			returns := ""
			isAsync := false
			for i := 0; i < int(node.ChildCount()); i++ {
				c := node.Child(uint(i))
				if c == nil { continue }
				if c.Kind() == "function_value_parameters" {
					params = NodeSource(kp.File, c)
				} else if c.Kind() == "user_type" || c.Kind() == "nullable_type" {
					returns = NodeSource(kp.File, c)
				} else if c.Kind() == "modifiers" {
					// Check for 'suspend' in function declaration
					modTxt := NodeSource(kp.File, c)
					if strings.Contains(modTxt, "suspend") {
						isAsync = true
					}
				}
			}

			doc := kp.getLeadingComment(node)
			cleanedDoc, aud, comp := parseAndCleanTags(doc)

			symKind := store.SymFunction
			if parent != "" {
				symKind = store.SymMethod
			}

			source.AddSymbol(store.Symbol{
				Name:          name,
				Kind:          symKind,
				Parent:        parent,
				File:          kp.FileName,
				Line:          int(node.StartPosition().Row + 1),
				Package:       kp.Package,
				Params:        params,
				Returns:       returns,
				Doc:           cleanedDoc,
				Audience:      aud,
				Compatibility: comp,
				Complexity:    1 + kp.getComplexity(node),
				IsAsync:       isAsync,
				SpawnsThread:  kp.hasThreadCreation(node),
			})

			// Extract call graph
			fullName := name
			if parent != "" {
				fullName = parent + "." + name
			}
			if kp.Package != "" && kp.Package != "main" {
				fullName = kp.Package + "." + fullName
			}
			kp.findCalls(node, fullName, source)
			return
		}
	}

	// Handle Field/Property
	if kind == "property_declaration" {
		varName := ""
		varType := ""
		// Look for variable_declaration inside property
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(uint(i))
			if c != nil && c.Kind() == "variable_declaration" {
				for j := 0; j < int(c.ChildCount()); j++ {
					vc := c.Child(uint(j))
					if vc != nil && vc.Kind() == "identifier" {
						varName = NodeSource(kp.File, vc)
					} else if vc != nil && (vc.Kind() == "user_type" || vc.Kind() == "nullable_type") {
						varType = NodeSource(kp.File, vc)
					}
				}
				break
			}
		}

		if varName != "" {
			source.AddSymbol(store.Symbol{
				Name:    varName,
				Kind:    store.SymField,
				Parent:  parent,
				File:    kp.FileName,
				Line:    int(node.StartPosition().Row + 1),
				Package: kp.Package,
				Type:    varType,
			})
			return
		}
	}

	// Generic recursive traversal for other nodes at root or unhandled scopes
	for i := 0; i < int(node.ChildCount()); i++ {
		kp.parseNode(node.Child(uint(i)), parent, source)
	}
}

func (kp *KotlinParser) parsePrimaryConstructor(node *tree_sitter.Node, parentClass string, source *store.Source) {
	// Look for class_parameters -> class_parameter
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(uint(i))
		if c == nil { continue }
		if c.Kind() == "class_parameters" {
			for j := 0; j < int(c.ChildCount()); j++ {
				p := c.Child(uint(j))
				if p != nil && p.Kind() == "class_parameter" {
					// Only extract as property if it defines 'val' or 'var'
					hasMod := false
					propName := ""
					propType := ""
					for k := 0; k < int(p.ChildCount()); k++ {
						pc := p.Child(uint(k))
						if pc == nil { continue }
						if pc.Kind() == "val" || pc.Kind() == "var" {
							hasMod = true
						} else if pc.Kind() == "identifier" {
							propName = NodeSource(kp.File, pc)
						} else if pc.Kind() == "user_type" || pc.Kind() == "nullable_type" {
							propType = NodeSource(kp.File, pc)
						}
					}
					if hasMod && propName != "" {
						source.AddSymbol(store.Symbol{
							Name:    propName,
							Kind:    store.SymField,
							Parent:  parentClass,
							File:    kp.FileName,
							Line:    int(p.StartPosition().Row + 1),
							Package: kp.Package,
							Type:    propType,
						})
					}
				}
			}
		}
	}
}

func (kp *KotlinParser) extractRelations(node *tree_sitter.Node) []string {
	var relations []string
	// Iterate delegation_specifier children
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(uint(i))
		if c != nil && c.Kind() == "delegation_specifier" {
			// Deep search for first identifier inside user_type
			target := ""
			kp.walkToKind(c, "user_type", func(ut *tree_sitter.Node) {
				for j := 0; j < int(ut.ChildCount()); j++ {
					utc := ut.Child(uint(j))
					if utc != nil && utc.Kind() == "identifier" {
						target = NodeSource(kp.File, utc)
						break
					}
				}
			})
			if target != "" {
				relations = append(relations, target)
			}
		}
	}
	return relations
}

func (kp *KotlinParser) walkToKind(node *tree_sitter.Node, kind string, cb func(*tree_sitter.Node)) {
	if node == nil { return }
	if node.Kind() == kind {
		cb(node)
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		kp.walkToKind(node.Child(uint(i)), kind, cb)
	}
}

func (kp *KotlinParser) findCalls(node *tree_sitter.Node, callerName string, source *store.Source) {
	if node == nil { return }

	if node.Kind() == "call_expression" {
		// Call often has identifier child directly for simple call
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(uint(i))
			if c != nil && c.Kind() == "identifier" {
				source.AddCall(callerName, NodeSource(kp.File, c))
				break
			} else if c != nil && c.Kind() == "navigation_expression" {
				// Chained calls obj.method()
				kp.extractNavigationCall(c, callerName, source)
				break
			}
		}
	}
	
	for i := 0; i < int(node.ChildCount()); i++ {
		kp.findCalls(node.Child(uint(i)), callerName, source)
	}
}

func (kp *KotlinParser) extractNavigationCall(node *tree_sitter.Node, caller string, source *store.Source) {
	// Typically node has navigation_suffix which contains simple_identifier
	// For simplicity, we can concatenate children or extract elements.
	// Just capture raw navigation text minus params as a call!
	source.AddCall(caller, NodeSource(kp.File, node))
}

func (kp *KotlinParser) getComplexity(node *tree_sitter.Node) int {
	if node == nil { return 0 }
	complexity := 0
	k := node.Kind()
	if k == "if_expression" || k == "for_statement" || k == "while_statement" || k == "when_expression" || k == "catch_clause" {
		complexity++
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		complexity += kp.getComplexity(node.Child(uint(i)))
	}
	return complexity
}

func (kp *KotlinParser) getLeadingComment(node *tree_sitter.Node) string {
	var comments []string
	prev := node.PrevSibling()
	for prev != nil {
		kind := prev.Kind()
		if strings.Contains(kind, "comment") {
			txt := strings.TrimSpace(NodeSource(kp.File, prev))
			txt = strings.TrimPrefix(txt, "//")
			txt = strings.TrimPrefix(txt, "/*")
			txt = strings.TrimSuffix(txt, "*/")
			lines := strings.Split(txt, "\n")
			for k, l := range lines {
				lines[k] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "*"))
			}
			comments = append([]string{strings.Join(lines, "\n")}, comments...)
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

func calculateKotlinTypeSize(typeStr string) int {
	t := strings.TrimSpace(typeStr)
	if strings.HasSuffix(t, "?") {
		// Nullable types usually result in boxed references, 8 bytes
		return 8
	}
	switch t {
	case "Byte", "Boolean":
		return 1
	case "Short", "Char":
		return 2
	case "Int", "Float":
		return 4
	case "Long", "Double":
		return 8
	}
	// Arrays and custom Objects are references (8 bytes)
	return 8
}

func (kp *KotlinParser) hasThreadCreation(node *tree_sitter.Node) bool {
	if node == nil { return false }
	
	if node.Kind() == "call_expression" {
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(uint(i))
			if c != nil && c.Kind() == "identifier" {
				ident := NodeSource(kp.File, c)
				if ident == "thread" || ident == "launch" || ident == "async" || ident == "runBlocking" {
					return true
				}
			}
		}
	}
	
	for i := 0; i < int(node.ChildCount()); i++ {
		if kp.hasThreadCreation(node.Child(uint(i))) {
			return true
		}
	}
	return false
}

func (kp *KotlinParser) calculateShallowSize(node *tree_sitter.Node) int {
	total := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		if child == nil { continue }
		
		if child.Kind() == "primary_constructor" {
			total += kp.sumConstructorSize(child)
		} else if child.Kind() == "class_body" {
			for j := 0; j < int(child.ChildCount()); j++ {
				bc := child.Child(uint(j))
				if bc != nil && bc.Kind() == "property_declaration" {
					total += kp.sumPropertySize(bc)
				}
			}
		}
	}
	return total
}

func (kp *KotlinParser) sumConstructorSize(node *tree_sitter.Node) int {
	total := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(uint(i))
		if c != nil && c.Kind() == "class_parameters" {
			for j := 0; j < int(c.ChildCount()); j++ {
				p := c.Child(uint(j))
				if p != nil && p.Kind() == "class_parameter" {
					hasMod := false
					pType := ""
					for k := 0; k < int(p.ChildCount()); k++ {
						pc := p.Child(uint(k))
						if pc == nil { continue }
						if pc.Kind() == "val" || pc.Kind() == "var" {
							hasMod = true
						} else if pc.Kind() == "user_type" || pc.Kind() == "nullable_type" {
							pType = NodeSource(kp.File, pc)
						}
					}
					if hasMod {
						total += calculateKotlinTypeSize(pType)
					}
				}
			}
		}
	}
	return total
}

func (kp *KotlinParser) sumPropertySize(node *tree_sitter.Node) int {
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(uint(i))
		if c != nil && c.Kind() == "variable_declaration" {
			for j := 0; j < int(c.ChildCount()); j++ {
				vc := c.Child(uint(j))
				if vc != nil && (vc.Kind() == "user_type" || vc.Kind() == "nullable_type") {
					return calculateKotlinTypeSize(NodeSource(kp.File, vc))
				}
			}
		}
	}
	// Default reference size for inferred property or unknown
	return 8
}
