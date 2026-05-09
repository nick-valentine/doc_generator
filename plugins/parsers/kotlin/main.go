package main

import (
	"doc_generator/pkg/store"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// Parser is the exported parser implementation
var Parser store.Parser = &KotlinParser{}

// Extensions is the list of file extensions this parser handles
var Extensions = []string{".kt"}

// NodeSource extracts the raw source string corresponding to the start and end byte of a Tree-Sitter Node.
func NodeSource(source []byte, node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}

// KotlinParser implements the store.Parser interface to extract declarations from Kotlin files.
type KotlinParser struct {
	FileName string
	File     []byte
	Package  string
}

// Parse extracts all classes, functions, and call graphs from the Kotlin file.
func (kp *KotlinParser) Parse(filePath string, fileContent []byte, source *store.Source) error {
	kp.FileName = filePath
	kp.File = fileContent
	source.AddFile(kp.FileName)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_java.Language()))

	tree := parser.Parse(kp.File, nil)
	defer tree.Close()

	root := tree.RootNode()

	// Find package name: Look for package keyword followed by scoped identifier/identifier
	// We can also fallback to finding a line with "package" prefix
	lines := strings.Split(string(kp.File), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			kp.Package = strings.TrimSuffix(strings.TrimPrefix(trimmed, "package "), ";")
			kp.Package = strings.TrimSpace(kp.Package)
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
	if kp.Package == "" {
		kp.Package = "main"
	}

	kp.parseNode(root, "", source)
	return nil
}

// parseNode is a recursive helper that traverses the AST, dispatching recognized declarations.
func (kp *KotlinParser) parseNode(node *tree_sitter.Node, currentClass string, source *store.Source) {
	if node == nil {
		return
	}

	nextClass := currentClass
	kind := node.Kind()

	// 1. Class declarations
	if kind == "class_declaration" || kind == "interface_declaration" {
		nodeText := NodeSource(kp.File, node)
		className := ""
		fields := strings.Fields(strings.TrimSpace(nodeText))
		for idx, field := range fields {
			if (field == "class" || field == "interface") && idx+1 < len(fields) {
				className = fields[idx+1]
				break
			}
		}
		if idx := strings.Index(className, ":"); idx != -1 {
			className = className[:idx]
		}
		if idx := strings.Index(className, "{"); idx != -1 {
			className = className[:idx]
		}
		if idx := strings.Index(className, "("); idx != -1 {
			className = className[:idx]
		}
		className = strings.TrimSpace(className)

		if className != "" {
			nextClass = className
			doc := kp.getLeadingComment(node)
			cleanedDoc, aud, comp := parseAndCleanTags(doc)
			relations := extractKotlinRelations(nodeText)

			kindToUse := store.SymStruct
			for _, f := range fields {
				if f == "interface" {
					kindToUse = store.SymInterface
					break
				}
			}

			source.AddSymbol(store.Symbol{
				Name:          nextClass,
				Kind:          kindToUse,
				File:          kp.FileName,
				Line:          int(node.StartPosition().Row + 1),
				Doc:           cleanedDoc,
				Audience:      aud,
				Compatibility: comp,
				Package:       kp.Package,
				Relations:     relations,
			})
		}
	} else {
		// Custom scanning of text for Kotlin specific constructs (like class/fun keywords)
		// when wrapped in ERROR nodes or not fully parsed by tree-sitter-java
		nodeText := NodeSource(kp.File, node)
		if strings.HasPrefix(strings.TrimSpace(nodeText), "class ") || strings.HasPrefix(strings.TrimSpace(nodeText), "interface ") {
			fields := strings.Fields(strings.TrimSpace(nodeText))
			className := ""
			for idx, field := range fields {
				if (field == "class" || field == "interface") && idx+1 < len(fields) {
					className = fields[idx+1]
					break
				}
			}
			if idx := strings.Index(className, ":"); idx != -1 {
				className = className[:idx]
			}
			if idx := strings.Index(className, "{"); idx != -1 {
				className = className[:idx]
			}
			if idx := strings.Index(className, "("); idx != -1 {
				className = className[:idx]
			}
			className = strings.TrimSpace(className)

			if className != "" {
				relations := extractKotlinRelations(nodeText)
				// Avoid duplicating
				exists := false
				for i, sym := range source.Symbols {
					if sym.Name == className && sym.File == kp.FileName {
						exists = true
						if len(source.Symbols[i].Relations) == 0 && len(relations) > 0 {
							source.Symbols[i].Relations = relations
						}
						break
					}
				}
				if !exists {
					kindToUse := store.SymStruct
					for _, f := range fields {
						if f == "interface" {
							kindToUse = store.SymInterface
							break
						}
					}
					nextClass = className
					source.AddSymbol(store.Symbol{
						Name:      className,
						Kind:      kindToUse,
						File:      kp.FileName,
						Line:      int(node.StartPosition().Row + 1),
						Package:   kp.Package,
						Relations: relations,
					})
				}
			}
		} else if strings.HasPrefix(strings.TrimSpace(nodeText), "fun ") {
			fields := strings.Fields(strings.TrimSpace(nodeText))
			if len(fields) > 1 {
				funcName := fields[1]
				if idx := strings.Index(funcName, "("); idx != -1 {
					funcName = funcName[:idx]
				}
				funcName = strings.TrimSpace(funcName)

				exists := false
				fullName := funcName
				if currentClass != "" {
					fullName = currentClass + "." + funcName
				}
				for _, sym := range source.Symbols {
					if sym.Name == fullName && sym.File == kp.FileName {
						exists = true
						break
					}
				}
				if !exists && funcName != "" {
					doc := kp.getLeadingComment(node)
					cleanedDoc, aud, comp := parseAndCleanTags(doc)

					complexity := kp.getComplexity(node) + 1
					startRow := node.StartPosition().Row
					endRow := node.EndPosition().Row
					lineCount := int(endRow - startRow + 1)

					kindSym := store.SymFunction
					if currentClass != "" {
						kindSym = store.SymMethod
					}

					source.AddSymbol(store.Symbol{
						Name:          fullName,
						Kind:          kindSym,
						File:          kp.FileName,
						Line:          int(node.StartPosition().Row + 1),
						Doc:           cleanedDoc,
						Audience:      aud,
						Compatibility: comp,
						Package:       kp.Package,
						Parent:        currentClass,
						LineCount:     lineCount,
						Complexity:    complexity,
					})

					caller := fullName
					if kp.Package != "" && kp.Package != "main" {
						caller = kp.Package + "." + caller
					}
					kp.findCalls(node, caller, source)
				}
			}
		}
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		kp.parseNode(node.Child(uint(i)), nextClass, source)
	}
}

// getLeadingComment finds contiguous comment nodes preceding the node.
func (kp *KotlinParser) getLeadingComment(node *tree_sitter.Node) string {
	var comments []string
	prev := node.PrevSibling()
	for prev != nil {
		kind := prev.Kind()
		if kind == "block_comment" || kind == "line_comment" || kind == "comment" {
			txt := strings.TrimSpace(NodeSource(kp.File, prev))
			txt = strings.TrimPrefix(txt, "//")
			txt = strings.TrimPrefix(txt, "/*")
			txt = strings.TrimSuffix(txt, "*/")
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

// findCalls recursively extracts all method calls in Kotlin.
func (kp *KotlinParser) findCalls(node *tree_sitter.Node, callerName string, source *store.Source) {
	if node == nil {
		return
	}

	// In Kotlin, we look for method_invocation or any identifiers followed by '('
	if node.Kind() == "method_invocation" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			methodName := NodeSource(kp.File, nameNode)
			objectNode := node.ChildByFieldName("object")
			if objectNode != nil {
				objectName := NodeSource(kp.File, objectNode)
				source.AddCall(callerName, objectName+"."+methodName)
			} else {
				source.AddCall(callerName, methodName)
			}
		}
	} else {
		kind := node.Kind()
		if kind != "class_declaration" && kind != "interface_declaration" && !strings.HasPrefix(strings.TrimSpace(NodeSource(kp.File, node)), "fun ") {
			nodeText := NodeSource(kp.File, node)
			if strings.Contains(nodeText, "(") {
				idx := strings.Index(nodeText, "(")
				prefix := strings.TrimSpace(nodeText[:idx])
				// Get the last field of prefix (e.g. obj.method)
				parts := strings.Fields(prefix)
				if len(parts) > 0 {
					potentialCall := parts[len(parts)-1]
					potentialCall = strings.TrimSpace(potentialCall)
					// Filter out keywords
					if potentialCall != "if" && potentialCall != "for" && potentialCall != "while" && potentialCall != "when" && potentialCall != "catch" && potentialCall != "fun" && potentialCall != "class" && potentialCall != "return" {
						if potentialCall != "" && !strings.Contains(potentialCall, " ") && !strings.Contains(potentialCall, "(") && !strings.Contains(potentialCall, ")") {
							source.AddCall(callerName, potentialCall)
						}
					}
				}
			}
		}
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		kp.findCalls(node.Child(uint(i)), callerName, source)
	}
}

// getComplexity estimates complexity based on control statements.
func (kp *KotlinParser) getComplexity(node *tree_sitter.Node) int {
	if node == nil {
		return 0
	}
	complexity := 0
	nodeText := NodeSource(kp.File, node)
	if strings.Contains(nodeText, "if ") || strings.Contains(nodeText, "for ") || strings.Contains(nodeText, "while ") || strings.Contains(nodeText, "when ") || strings.Contains(nodeText, "catch ") {
		complexity++
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		complexity += kp.getComplexity(node.Child(uint(i)))
	}
	return complexity
}

func extractKotlinRelations(nodeText string) []string {
	if idxBrace := strings.Index(nodeText, "{"); idxBrace != -1 {
		nodeText = nodeText[:idxBrace]
	}
	// Find the colon outside any parenthesis
	idx := -1
	parenDepth := 0
	for i, char := range nodeText {
		if char == '(' {
			parenDepth++
		} else if char == ')' {
			parenDepth--
		} else if char == ':' && parenDepth == 0 {
			idx = i
			break
		}
	}

	var relations []string
	if idx != -1 {
		rem := nodeText[idx+1:]
		parts := strings.Split(rem, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if idxParen := strings.Index(part, "("); idxParen != -1 {
				part = part[:idxParen]
			}
			part = strings.TrimSpace(part)
			if idxGen := strings.Index(part, "<"); idxGen != -1 {
				part = part[:idxGen]
			}
			part = strings.TrimSpace(part)
			part = strings.TrimFunc(part, func(r rune) bool {
				return r == ')' || r == '(' || r == ',' || r == ' '
			})
			if part != "" && !strings.Contains(part, " ") {
				relations = append(relations, part)
			}
		}
	}
	return relations
}
