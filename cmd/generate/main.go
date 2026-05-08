package main

import (
	"bytes"
	"doc_generator/pkg/generators"
	"doc_generator/pkg/store"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"plugin"
	"strings"
)

func main() {
	audienceFlag := flag.String("audience", "", "Filter symbols by audience (e.g. API, INTERNAL, USER, DEVELOPER)")
	formatFlag := flag.String("format", "html", "Output format: html or text")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run cmd/generate/main.go [-audience <tag>] [-format <html|text>] <input_directory>")
		os.Exit(1)
	}
	inputPath := args[0]

	source := store.Source{}

	parsersList, err := loadParserPlugins()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading parser plugins: %v\n", err)
		os.Exit(1)
	}

	err = filepath.WalkDir(inputPath, func(fPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Ignore hidden folders, .gopath, and the copied go-tree-sitter folder
		if d.IsDir() {
			name := d.Name()
			if (strings.HasPrefix(name, ".") && name != "." && name != "..") || name == "go-tree-sitter" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		fileType := path.Ext(fPath)
		fileContent, err := os.ReadFile(fPath)
		if err != nil {
			return nil
		}

		// Use dynamic parsers to process matching files
		for _, p := range parsersList {
			for _, ext := range p.Extensions {
				if fileType == ext {
					if ext == ".go" && strings.HasSuffix(fPath, "_test.go") {
						continue
					}
					_ = p.Parser.Parse(fPath, fileContent, &source)
				}
			}
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning files: %v\n", err)
		os.Exit(1)
	}

	// Filter by audience if specified
	if *audienceFlag != "" {
		filteredSymbols := source.FilterByAudience(*audienceFlag)
		source.Symbols = filteredSymbols
	}

	// Generate the static call graph and import graph images
	generateCallGraphs(&source)
	generateImportGraph(&source)
	generateFullProgramGraph(&source)
	generateRelationsGraph(&source)
	generateTypeGraphs(&source)

	generatorsList, err := loadGeneratorPlugins()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading generator plugins: %v\n", err)
		os.Exit(1)
	}

	var generator store.Generator
	targetFormat := strings.ToLower(*formatFlag)
	for _, g := range generatorsList {
		if g.Format == targetFormat {
			generator = g.Generator
			break
		}
	}

	if generator == nil {
		if targetFormat == "text" {
			generator = &generators.MarkdownGenerator{}
		} else {
			generator = &generators.HTMLGenerator{}
		}
	}

	output, err := generator.Generate(&source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating documentation: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(output)
}

type LoadedParser struct {
	Parser     store.Parser
	Extensions []string
}

func loadParserPlugins() ([]LoadedParser, error) {
	pluginsDir := "plugins/parsers"
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}

	var loaded []LoadedParser
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".so") {
			pPath := filepath.Join(pluginsDir, f.Name())
			p, err := plugin.Open(pPath)
			if err != nil {
				fmt.Printf("Warning: Failed to load parser plugin %s: %v\n", pPath, err)
				continue
			}

			parserSym, err := p.Lookup("Parser")
			if err != nil {
				fmt.Printf("Warning: Missing 'Parser' symbol in plugin %s: %v\n", pPath, err)
				continue
			}

			extsSym, err := p.Lookup("Extensions")
			if err != nil {
				fmt.Printf("Warning: Missing 'Extensions' symbol in plugin %s: %v\n", pPath, err)
				continue
			}

			parser, ok := parserSym.(store.Parser)
			if !ok {
				if ptr, ok := parserSym.(*store.Parser); ok {
					parser = *ptr
				} else {
					fmt.Printf("Warning: 'Parser' symbol in %s does not implement store.Parser interface\n", pPath)
					continue
				}
			}

			exts, ok := extsSym.(*[]string)
			if !ok {
				fmt.Printf("Warning: 'Extensions' symbol in %s is not of type *[]string\n", pPath)
				continue
			}

			loaded = append(loaded, LoadedParser{
				Parser:     parser,
				Extensions: *exts,
			})
		}
	}

	return loaded, nil
}

type LoadedGenerator struct {
	Generator store.Generator
	Format    string
}

func loadGeneratorPlugins() ([]LoadedGenerator, error) {
	pluginsDir := "plugins/generators"
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}

	var loaded []LoadedGenerator
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".so") {
			pPath := filepath.Join(pluginsDir, f.Name())
			p, err := plugin.Open(pPath)
			if err != nil {
				fmt.Printf("Warning: Failed to load generator plugin %s: %v\n", pPath, err)
				continue
			}

			genSym, err := p.Lookup("Generator")
			if err != nil {
				fmt.Printf("Warning: Missing 'Generator' symbol in plugin %s: %v\n", pPath, err)
				continue
			}

			formatSym, err := p.Lookup("Format")
			if err != nil {
				fmt.Printf("Warning: Missing 'Format' symbol in plugin %s: %v\n", pPath, err)
				continue
			}

			generator, ok := genSym.(store.Generator)
			if !ok {
				if ptr, ok := genSym.(*store.Generator); ok {
					generator = *ptr
				} else {
					fmt.Printf("Warning: 'Generator' symbol in %s does not implement store.Generator interface\n", pPath)
					continue
				}
			}

			format, ok := formatSym.(*string)
			if !ok {
				fmt.Printf("Warning: 'Format' symbol in %s is not of type *string\n", pPath)
				continue
			}

			loaded = append(loaded, LoadedGenerator{
				Generator: generator,
				Format:    *format,
			})
		}
	}

	return loaded, nil
}

// generateCallGraphs compiles static PNG call graphs for all methods/functions using the local Graphviz 'dot' utility.
func generateCallGraphs(source *store.Source) {
	imagesDir := filepath.Join("docs", "images")
	_ = os.MkdirAll(imagesDir, 0755)

	processed := make(map[string]bool)
	var keys []string
	for _, c := range source.Calls {
		if !processed[c.Caller] {
			processed[c.Caller] = true
			keys = append(keys, c.Caller)
		}
		if !processed[c.Callee] {
			processed[c.Callee] = true
			keys = append(keys, c.Callee)
		}
	}

	type edge struct{ From, To string }

	for _, key := range keys {
		// Collect callers recursively up to 5 levels
		visitedCallers := make(map[string]bool)
		var callerEdges []edge
		var collectCallers func(node string, depth int)
		collectCallers = func(node string, depth int) {
			if depth >= 5 || visitedCallers[node] {
				return
			}
			visitedCallers[node] = true
			callers := source.GetCallers(node)
			for _, caller := range callers {
				callerEdges = append(callerEdges, edge{From: caller, To: node})
				collectCallers(caller, depth+1)
			}
		}
		collectCallers(key, 0)

		// Collect callees recursively up to 5 levels
		visitedCallees := make(map[string]bool)
		var calleeEdges []edge
		var collectCallees func(node string, depth int)
		collectCallees = func(node string, depth int) {
			if depth >= 5 || visitedCallees[node] {
				return
			}
			visitedCallees[node] = true
			callees := source.GetCallees(node)
			for _, callee := range callees {
				calleeEdges = append(calleeEdges, edge{From: node, To: callee})
				collectCallees(callee, depth+1)
			}
		}
		collectCallees(key, 0)

		if len(callerEdges) == 0 && len(calleeEdges) == 0 {
			continue
		}

		cleanKey := strings.ReplaceAll(key, ".", "_")
		dotPath := filepath.Join(imagesDir, fmt.Sprintf("%s_call_graph.dot", cleanKey))
		pngPath := filepath.Join(imagesDir, fmt.Sprintf("%s_call_graph.png", cleanKey))

		var dot bytes.Buffer
		dot.WriteString("digraph G {\n")
		dot.WriteString("    rankdir=LR;\n")
		dot.WriteString("    node [shape=box, style=\"filled,rounded\", color=\"#4A90E2\", fontname=\"Helvetica\", fillcolor=\"#F0F4F8\"];\n")
		dot.WriteString("    edge [color=\"#999999\", fontname=\"Helvetica\", fontsize=10];\n")

		// Highlight focal node
		dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#4A90E2\", fontcolor=\"white\", style=\"filled,rounded,bold\"];\n", key))

		// Render unique callers and callees edges
		renderedEdges := make(map[edge]bool)
		for _, e := range callerEdges {
			if !renderedEdges[e] {
				renderedEdges[e] = true
				dot.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", e.From, e.To))
			}
		}
		for _, e := range calleeEdges {
			if !renderedEdges[e] {
				renderedEdges[e] = true
				dot.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", e.From, e.To))
			}
		}

		dot.WriteString("}\n")

		_ = os.WriteFile(dotPath, dot.Bytes(), 0644)

		cmd := exec.Command("dot", "-Tpng", "-o", pngPath, dotPath)
		_ = cmd.Run()

		_ = os.Remove(dotPath)
		_ = writeGraphHTML(filepath.Join("docs", "graphs", fmt.Sprintf("%s_call.html", cleanKey)), fmt.Sprintf("%s Call Graph", key), fmt.Sprintf("../images/%s_call_graph.png", cleanKey))
	}
}

// generateImportGraph compiles a visual package/file import relationship graph.
func generateImportGraph(source *store.Source) {
	imagesDir := filepath.Join("docs", "images")
	_ = os.MkdirAll(imagesDir, 0755)

	var imports []store.Symbol
	for _, sym := range source.Symbols {
		if sym.Kind == store.SymImport {
			imports = append(imports, sym)
		}
	}

	if len(imports) == 0 {
		return
	}

	dotPath := filepath.Join(imagesDir, "imports_graph.dot")
	pngPath := filepath.Join(imagesDir, "imports_graph.png")

	var dot bytes.Buffer
	dot.WriteString("digraph G {\n")
	dot.WriteString("    rankdir=LR;\n")
	dot.WriteString("    node [shape=box, style=\"filled,rounded\", color=\"#6366F1\", fontname=\"Helvetica\", fillcolor=\"#F5F3FF\", fontcolor=\"#1E1B4B\", fontsize=10];\n")
	dot.WriteString("    edge [color=\"#818CF8\", fontname=\"Helvetica\", fontsize=10];\n")

	for _, imp := range imports {
		fileBase := filepath.Base(imp.File)
		dot.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", fileBase, imp.Name))
	}

	dot.WriteString("}\n")

	_ = os.WriteFile(dotPath, dot.Bytes(), 0644)

	cmd := exec.Command("dot", "-Tpng", "-o", pngPath, dotPath)
	_ = cmd.Run()

	_ = os.Remove(dotPath)
	_ = writeGraphHTML(filepath.Join("docs", "graphs", "imports.html"), "Import Dependency Graph", "../images/imports_graph.png")
}

// generateFullProgramGraph compiles a visual callee graph of the entire program starting from main.
func generateFullProgramGraph(source *store.Source) {
	imagesDir := filepath.Join("docs", "images")
	_ = os.MkdirAll(imagesDir, 0755)

	dotPath := filepath.Join(imagesDir, "program_graph.dot")
	pngPath := filepath.Join(imagesDir, "program_graph.png")

	type edge struct{ From, To string }
	var programEdges []edge
	visited := make(map[string]bool)

	var walk func(node string)
	walk = func(node string) {
		if visited[node] {
			return
		}
		visited[node] = true
		callees := source.GetCallees(node)
		for _, callee := range callees {
			programEdges = append(programEdges, edge{From: node, To: callee})
			walk(callee)
		}
	}
	walk("main")

	var dot bytes.Buffer
	dot.WriteString("digraph G {\n")
	dot.WriteString("    rankdir=LR;\n")
	dot.WriteString("    node [shape=box, style=\"filled,rounded\", color=\"#10B981\", fontname=\"Helvetica\", fillcolor=\"#ECFDF5\", fontcolor=\"#064E3B\", fontsize=10];\n")
	dot.WriteString("    edge [color=\"#34D399\", fontname=\"Helvetica\", fontsize=10];\n")

	// Highlight main node
	dot.WriteString("    \"main\" [fillcolor=\"#10B981\", fontcolor=\"white\", style=\"filled,rounded,bold\"];\n")

	renderedEdges := make(map[edge]bool)
	for _, e := range programEdges {
		if !renderedEdges[e] {
			renderedEdges[e] = true
			dot.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", e.From, e.To))
		}
	}

	dot.WriteString("}\n")

	_ = os.WriteFile(dotPath, dot.Bytes(), 0644)

	cmd := exec.Command("dot", "-Tpng", "-o", pngPath, dotPath)
	_ = cmd.Run()

	_ = os.Remove(dotPath)
	_ = writeGraphHTML(filepath.Join("docs", "graphs", "program.html"), "Full Program Callee Graph", "../images/program_graph.png")
}

// generateRelationsGraph compiles a visual type relationship diagram (implements, embeds, composition).
func generateRelationsGraph(source *store.Source) {
	imagesDir := filepath.Join("docs", "images")
	_ = os.MkdirAll(imagesDir, 0755)

	dotPath := filepath.Join(imagesDir, "relations_graph.dot")
	pngPath := filepath.Join(imagesDir, "relations_graph.png")

	var dot bytes.Buffer
	dot.WriteString("digraph G {\n")
	dot.WriteString("    rankdir=BT;\n")
	dot.WriteString("    node [shape=box, style=\"filled,rounded\", color=\"#818CF8\", fontname=\"Helvetica\", fillcolor=\"#EEF2FF\", fontcolor=\"#312E81\", fontsize=10];\n")
	dot.WriteString("    edge [fontname=\"Helvetica\", fontsize=9];\n")

	var structs []store.Symbol
	var interfaces []store.Symbol
	for _, sym := range source.Symbols {
		if sym.Kind == store.SymStruct {
			structs = append(structs, sym)
		} else if sym.Kind == store.SymInterface {
			interfaces = append(interfaces, sym)
		}
	}

	// Render Interfaces with distinct styling
	for _, it := range interfaces {
		dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#ECFDF5\", color=\"#10B981\", fontcolor=\"#064E3B\", style=\"filled,rounded,dashed\"];\n", it.Name))
	}

	renderedRelations := make(map[string]bool)

	for _, s := range structs {
		fields := source.GetStructFields(s.Name)
		methods := source.GetStructMethods(s.Name)

		// Implements Relationships
		for _, m := range methods {
			if m.Name == "Parse" {
				rel := fmt.Sprintf("    \"%s\" -> \"Parser\" [arrowhead=onormal, style=dashed, color=\"#10B981\", label=\"implements\"];\n", s.Name)
				if !renderedRelations[rel] {
					renderedRelations[rel] = true
					dot.WriteString(rel)
				}
			}
			if m.Name == "Generate" {
				rel := fmt.Sprintf("    \"%s\" -> \"Generator\" [arrowhead=onormal, style=dashed, color=\"#10B981\", label=\"implements\"];\n", s.Name)
				if !renderedRelations[rel] {
					renderedRelations[rel] = true
					dot.WriteString(rel)
				}
			}
		}

		// Embedding / Composition Relationships
		for _, f := range fields {
			// Find if field type matches a parsed struct
			cleanType := strings.Trim(f.Type, "*[]")
			if idx := strings.LastIndex(cleanType, "."); idx != -1 {
				cleanType = cleanType[idx+1:]
			}

			isStruct := false
			for _, other := range structs {
				if other.Name == cleanType {
					isStruct = true
					break
				}
			}

			if isStruct {
				if f.Name == f.Type || strings.HasSuffix(f.Type, f.Name) {
					// Embedding
					rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=empty, style=solid, color=\"#818CF8\", label=\"embeds\"];\n", s.Name, cleanType)
					if !renderedRelations[rel] {
						renderedRelations[rel] = true
						dot.WriteString(rel)
					}
				} else {
					// Field Composition
					rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=diamond, style=solid, color=\"#F59E0B\", label=\"composed of\"];\n", s.Name, cleanType)
					if !renderedRelations[rel] {
						renderedRelations[rel] = true
						dot.WriteString(rel)
					}
				}
			}
		}
	}

	dot.WriteString("}\n")

	_ = os.WriteFile(dotPath, dot.Bytes(), 0644)

	cmd := exec.Command("dot", "-Tpng", "-o", pngPath, dotPath)
	_ = cmd.Run()

	_ = writeGraphHTML(filepath.Join("docs", "graphs", "relations.html"), "Type Relationships Graph", "../images/relations_graph.png")
}

// writeGraphHTML creates a premium standalone HTML wrapper for displaying a large Graphviz visualization.
func writeGraphHTML(outputPath, title, imageRelPath string) error {
	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%[1]s | Visual Graph Viewer</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-primary: #0F172A;
            --bg-secondary: #1E293B;
            --accent-primary: #6366F1;
            --text-primary: #F8FAFC;
            --text-secondary: #94A3B8;
            --border-color: rgba(255, 255, 255, 0.08);
        }
        body {
            background-color: var(--bg-primary);
            color: var(--text-primary);
            font-family: 'Outfit', sans-serif;
            margin: 0;
            padding: 2rem;
            display: flex;
            flex-direction: column;
            align-items: center;
            min-height: 100vh;
            box-sizing: border-box;
        }
        header {
            width: 100%%;
            max-width: 1200px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 2rem;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 1rem;
        }
        h1 {
            font-size: 1.8rem;
            font-weight: 600;
            margin: 0;
            background: linear-gradient(135deg, #818CF8, #34D399);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .back-btn {
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
            padding: 0.6rem 1.2rem;
            border-radius: 8px;
            text-decoration: none;
            font-size: 0.9rem;
            font-weight: 500;
            transition: all 0.2s;
        }
        .back-btn:hover {
            background: rgba(255, 255, 255, 0.1);
            color: var(--text-primary);
            border-color: var(--accent-primary);
            transform: translateX(-4px);
        }
        .graph-container {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 2rem;
            display: flex;
            justify-content: center;
            align-items: center;
            width: 100%%;
            max-width: 1200px;
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.3);
            overflow: auto;
        }
        img {
            max-width: 100%%;
            height: auto;
            border-radius: 8px;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
        }
    </style>
</head>
<body>
    <header>
        <h1>%[1]s</h1>
        <a href="../index.html" class="back-btn">← Back to Dashboard</a>
    </header>
    <div class="graph-container">
        <img src="%[2]s" alt="%[1]s Image">
    </div>
</body>
</html>`, title, imageRelPath)

	return os.WriteFile(outputPath, []byte(html), 0644)
}

// generateTypeGraphs compiles static PNG type relationship graphs for each struct/interface.
func generateTypeGraphs(source *store.Source) {
	imagesDir := filepath.Join("docs", "images")
	graphsDir := filepath.Join("docs", "graphs")
	_ = os.MkdirAll(imagesDir, 0755)
	_ = os.MkdirAll(graphsDir, 0755)

	var structs []store.Symbol
	var interfaces []store.Symbol
	for _, sym := range source.Symbols {
		if sym.Kind == store.SymStruct {
			structs = append(structs, sym)
		} else if sym.Kind == store.SymInterface {
			interfaces = append(interfaces, sym)
		}
	}

	for _, s := range structs {
		dotPath := filepath.Join(imagesDir, fmt.Sprintf("%s_type_graph.dot", s.Name))
		pngPath := filepath.Join(imagesDir, fmt.Sprintf("%s_type_graph.png", s.Name))
		htmlPath := filepath.Join(graphsDir, fmt.Sprintf("%s_type.html", s.Name))

		var dot bytes.Buffer
		dot.WriteString("digraph G {\n")
		dot.WriteString("    rankdir=BT;\n")
		dot.WriteString("    node [shape=box, style=\"filled,rounded\", color=\"#818CF8\", fontname=\"Helvetica\", fillcolor=\"#EEF2FF\", fontcolor=\"#312E81\", fontsize=10];\n")
		dot.WriteString("    edge [fontname=\"Helvetica\", fontsize=9];\n")

		// Highlight focal struct node
		dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#818CF8\", fontcolor=\"white\", style=\"filled,rounded,bold\"];\n", s.Name))

		fields := source.GetStructFields(s.Name)
		methods := source.GetStructMethods(s.Name)

		renderedRelations := make(map[string]bool)

		// Implements
		for _, m := range methods {
			if m.Name == "Parse" {
				dot.WriteString("    \"Parser\" [fillcolor=\"#ECFDF5\", color=\"#10B981\", fontcolor=\"#064E3B\", style=\"filled,rounded,dashed\"];\n")
				rel := fmt.Sprintf("    \"%s\" -> \"Parser\" [arrowhead=onormal, style=dashed, color=\"#10B981\", label=\"implements\"];\n", s.Name)
				if !renderedRelations[rel] {
					renderedRelations[rel] = true
					dot.WriteString(rel)
				}
			}
			if m.Name == "Generate" {
				dot.WriteString("    \"Generator\" [fillcolor=\"#ECFDF5\", color=\"#10B981\", fontcolor=\"#064E3B\", style=\"filled,rounded,dashed\"];\n")
				rel := fmt.Sprintf("    \"%s\" -> \"Generator\" [arrowhead=onormal, style=dashed, color=\"#10B981\", label=\"implements\"];\n", s.Name)
				if !renderedRelations[rel] {
					renderedRelations[rel] = true
					dot.WriteString(rel)
				}
			}
		}

		// Composition / Embedding
		for _, f := range fields {
			cleanType := strings.Trim(f.Type, "*[]")
			if idx := strings.LastIndex(cleanType, "."); idx != -1 {
				cleanType = cleanType[idx+1:]
			}

			isStruct := false
			for _, other := range structs {
				if other.Name == cleanType {
					isStruct = true
					break
				}
			}

			if isStruct {
				if f.Name == f.Type || strings.HasSuffix(f.Type, f.Name) {
					rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=empty, style=solid, color=\"#818CF8\", label=\"embeds\"];\n", s.Name, cleanType)
					if !renderedRelations[rel] {
						renderedRelations[rel] = true
						dot.WriteString(rel)
					}
				} else {
					rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=diamond, style=solid, color=\"#F59E0B\", label=\"composed of\"];\n", s.Name, cleanType)
					if !renderedRelations[rel] {
						renderedRelations[rel] = true
						dot.WriteString(rel)
					}
				}
			}
		}

		dot.WriteString("}\n")
		_ = os.WriteFile(dotPath, dot.Bytes(), 0644)

		cmd := exec.Command("dot", "-Tpng", "-o", pngPath, dotPath)
		_ = cmd.Run()
		_ = os.Remove(dotPath)

		_ = writeGraphHTML(htmlPath, fmt.Sprintf("%s Type Relationship Graph", s.Name), fmt.Sprintf("../images/%s_type_graph.png", s.Name))
	}

	for _, s := range interfaces {
		dotPath := filepath.Join(imagesDir, fmt.Sprintf("%s_type_graph.dot", s.Name))
		pngPath := filepath.Join(imagesDir, fmt.Sprintf("%s_type_graph.png", s.Name))
		htmlPath := filepath.Join(graphsDir, fmt.Sprintf("%s_type.html", s.Name))

		var dot bytes.Buffer
		dot.WriteString("digraph G {\n")
		dot.WriteString("    rankdir=BT;\n")
		dot.WriteString("    node [shape=box, style=\"filled,rounded\", color=\"#818CF8\", fontname=\"Helvetica\", fillcolor=\"#EEF2FF\", fontcolor=\"#312E81\", fontsize=10];\n")
		dot.WriteString("    edge [fontname=\"Helvetica\", fontsize=9];\n")

		// Highlight focal interface node
		dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#ECFDF5\", color=\"#10B981\", fontcolor=\"#064E3B\", style=\"filled,rounded,dashed,bold\"];\n", s.Name))

		// Find structs that implement this interface
		for _, other := range structs {
			methods := source.GetStructMethods(other.Name)
			for _, m := range methods {
				if (s.Name == "Parser" && m.Name == "Parse") || (s.Name == "Generator" && m.Name == "Generate") {
					dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#EEF2FF\", color=\"#818CF8\", fontcolor=\"#312E81\", style=\"filled,rounded\"];\n", other.Name))
					dot.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=onormal, style=dashed, color=\"#10B981\", label=\"implements\"];\n", other.Name, s.Name))
				}
			}
		}

		dot.WriteString("}\n")
		_ = os.WriteFile(dotPath, dot.Bytes(), 0644)

		cmd := exec.Command("dot", "-Tpng", "-o", pngPath, dotPath)
		_ = cmd.Run()
		_ = os.Remove(dotPath)

		_ = writeGraphHTML(htmlPath, fmt.Sprintf("%s Interface Implementations", s.Name), fmt.Sprintf("../images/%s_type_graph.png", s.Name))
	}
}

