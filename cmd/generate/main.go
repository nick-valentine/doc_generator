package main

import (
	"bytes"
	"crypto/md5"
	"doc_generator/pkg/analysis"
	"doc_generator/pkg/diagram"
	"doc_generator/pkg/generators"
	"doc_generator/pkg/store"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"plugin"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Concurrency     int    `toml:"concurrency"`
	DiagramProvider string `toml:"diagram_provider"`
	MaxDiagramSymbols int  `toml:"max_diagram_symbols"`
	Input struct {
		Directory string   `toml:"directory"`
		Ignore    []string `toml:"ignore"`
	} `toml:"input"`
	Output []struct {
		Format    string `toml:"format"`
		Directory string `toml:"directory"`
	} `toml:"output"`
}

type DiagramJob struct {
	DotContent  string
	PumlContent string
	PngPath     string
	PostRun     func()
}

func getMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

func main() {
	configFlag := flag.String("config", "docgen.toml", "Path to TOML configuration file")
	audienceFlag := flag.String("audience", "", "Filter symbols by audience (e.g. API, INTERNAL, USER, DEVELOPER)")
	concurrencyFlag := flag.Int("concurrency", 0, "Number of concurrent workers (defaults to number of CPUs or TOML config)")
	flag.Parse()

	configData, err := os.ReadFile(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config file %s: %v\n", *configFlag, err)
		os.Exit(1)
	}

	var config Config
	if err := toml.Unmarshal(configData, &config); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
		os.Exit(1)
	}

	if config.Input.Directory == "" {
		fmt.Fprintf(os.Stderr, "Error: input.directory must be specified in config\n")
		os.Exit(1)
	}

	source := store.Source{}

	parsersList, err := loadParserPlugins()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading parser plugins: %v\n", err)
		os.Exit(1)
	}

	err = filepath.WalkDir(config.Input.Directory, func(fPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			if (strings.HasPrefix(name, ".") && name != "." && name != "..") || name == "go-tree-sitter" || name == "vendor" {
				return filepath.SkipDir
			}
			for _, ign := range config.Input.Ignore {
				if name == ign {
					return filepath.SkipDir
				}
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

	// Look for coverage report
	coverageFile := ""
	standardFiles := []string{
		filepath.Join(config.Input.Directory, "coverage.out"),
		filepath.Join(config.Input.Directory, "cover.out"),
		filepath.Join(config.Input.Directory, "coverage.info"),
		filepath.Join(config.Input.Directory, "lcov.info"),
		filepath.Join(config.Input.Directory, "coverage.lcov"),
		filepath.Join(config.Input.Directory, "coverage.ccov"),
		"coverage.out",
		"cover.out",
		"coverage.info",
		"lcov.info",
		"coverage.lcov",
		"coverage.ccov",
	}
	for _, f := range standardFiles {
		if _, err := os.Stat(f); err == nil {
			coverageFile = f
			break
		}
	}
	if coverageFile != "" {
		fmt.Printf("Found code coverage report: %s. Loading metrics...\n", coverageFile)
		blocks, err := store.ParseCoverage(coverageFile)
		if err == nil {
			source.ApplyCoverage(blocks)
			fmt.Println("Successfully applied code coverage metrics to source symbols.")
		} else {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse coverage report: %v\n", err)
		}
	}

	// Filter by audience if specified
	if *audienceFlag != "" {
		filteredSymbols := source.FilterByAudience(*audienceFlag)
		source.Symbols = filteredSymbols
	}
	
	// Run automated Design Pattern analysis
	fmt.Println("Running static heuristics pass to detect Design Patterns...")
	analysis.RunPatternAnalysis(&source)
	analysis.RunNetworkAnalysis(&source)
	fmt.Printf("Completed heuristic analysis. Detected %d patterns, %d network components.\n", len(source.Patterns), len(source.NetworkAnalysis))

	var outputDirs []string
	for _, out := range config.Output {
		outputDirs = append(outputDirs, out.Directory)
		_ = os.MkdirAll(out.Directory, 0755)
	}

	// Dynamic Diagram Provider and Parallel Rendering Threadpool
	var provider diagram.DiagramProvider
	if config.DiagramProvider == "none" {
		fmt.Println("Disabling diagram generation via configuration")
		provider = nil
	} else if config.DiagramProvider != "" {
		provider = diagram.GetProviderByName(config.DiagramProvider)
		if provider == nil {
			fmt.Printf("Warning: Configured diagram provider %q not available, falling back to auto-detection\n", config.DiagramProvider)
			provider = diagram.GetBestProvider()
		}
	} else {
		provider = diagram.GetBestProvider()
	}

	if provider != nil {
		fmt.Printf("Using diagram provider: %s\n", provider.Name())
		var jobs []DiagramJob
		inputGenStart := time.Now()
		limit := config.MaxDiagramSymbols
		if limit <= 0 {
			limit = 1000 // Default safety limit
		}

		if len(source.Symbols) <= limit {
			generateCallGraphs(&source, outputDirs, &jobs)
			generateImportGraph(&source, outputDirs, &jobs)
			generateFullProgramGraph(&source, outputDirs, &jobs)
			generateRelationsGraph(&source, outputDirs, &jobs)
			generateTypeGraphs(&source, outputDirs, &jobs)
			generateSequenceDiagrams(&source, outputDirs, &jobs)
			generateTimingDiagrams(&source, outputDirs, &jobs)
		} else {
			fmt.Printf("Skipping bulky component diagrams (>%d symbols) as count is %d. Configure max_diagram_symbols in docgen.toml to override.\n", limit, len(source.Symbols))
		}
		// Always run requested Pattern & Network Generator
		generatePatternGraphs(&source, outputDirs, &jobs)
		generateNetworkGraphs(&source, outputDirs, &jobs)
		fmt.Printf("Input generation completed in %v. Total jobs: %d.\n", time.Since(inputGenStart), len(jobs))

		type diagramCacheType struct {
			mu     sync.Mutex
			Hashes map[string]string
		}
		var cache diagramCacheType
		cache.Hashes = make(map[string]string)
		cachePath := "docs/diagram_cache.json"
		if data, err := os.ReadFile(cachePath); err == nil {
			_ = json.Unmarshal(data, &cache.Hashes)
		}

		fmt.Printf("Generating %d diagrams concurrently...\n", len(jobs))

		numWorkers := runtime.NumCPU()
		if *concurrencyFlag > 0 {
			numWorkers = *concurrencyFlag
		} else if config.Concurrency > 0 {
			numWorkers = config.Concurrency
		}

		if numWorkers > len(jobs) {
			numWorkers = len(jobs)
		}

		jobsChan := make(chan DiagramJob, len(jobs))
		for _, job := range jobs {
			jobsChan <- job
		}
		close(jobsChan)

		var completedCount int32
		totalJobs := int32(len(jobs))

		startTime := time.Now()
		var wg sync.WaitGroup
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobsChan {
					var content string
					if provider.Name() == "PlantUML" && job.PumlContent != "" {
						content = job.PumlContent
					} else {
						content = job.DotContent
					}

					hash := getMD5Hash(content)

					cache.mu.Lock()
					cachedHash, exists := cache.Hashes[job.PngPath]
					cache.mu.Unlock()

					_, fileErr := os.Stat(job.PngPath)
					if exists && fileErr == nil && cachedHash == hash {
						current := atomic.AddInt32(&completedCount, 1)
						if current%100 == 0 || current == totalJobs {
							elapsed := time.Since(startTime).Seconds()
							rate := 0.0
							if elapsed > 0 {
								rate = float64(current) / elapsed
							}
							fmt.Printf("Progress: %d/%d diagrams generated (%.1f%%) | %.1f diagrams/sec (cached)\n", current, totalJobs, float64(current)/float64(totalJobs)*100, rate)
						}
						continue
					}

					err := provider.Generate(content, job.PngPath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to generate diagram %s: %v\n", job.PngPath, err)
					} else {
						cache.mu.Lock()
						cache.Hashes[job.PngPath] = hash
						cache.mu.Unlock()
						if job.PostRun != nil {
							job.PostRun()
						}
					}
					
					current := atomic.AddInt32(&completedCount, 1)
					if current%100 == 0 || current == totalJobs {
						elapsed := time.Since(startTime).Seconds()
						rate := 0.0
						if elapsed > 0 {
							rate = float64(current) / elapsed
						}
						fmt.Printf("Progress: %d/%d diagrams generated (%.1f%%) | %.1f diagrams/sec\n", current, totalJobs, float64(current)/float64(totalJobs)*100, rate)
					}
				}
			}()
		}
		wg.Wait()

		cache.mu.Lock()
		if data, err := json.MarshalIndent(cache.Hashes, "", "  "); err == nil {
			_ = os.MkdirAll("docs", 0755)
			_ = os.WriteFile(cachePath, data, 0644)
		}
		cache.mu.Unlock()

		fmt.Println("Concurrently generated diagrams successfully.")
	} else {
		fmt.Println("Warning: No diagram provider (dot or plantuml) found in PATH. Skipping diagram generation.")
	}

	generatorsList, err := loadGeneratorPlugins()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading generator plugins: %v\n", err)
		os.Exit(1)
	}

	for _, out := range config.Output {
		var generator store.Generator
		targetFormat := strings.ToLower(out.Format)
		for _, g := range generatorsList {
			if g.Format == targetFormat {
				generator = g.Generator
				break
			}
		}

		if generator == nil {
			if targetFormat == "text" || targetFormat == "markdown" {
				generator = &generators.MarkdownGenerator{}
			} else {
				generator = &generators.HTMLGenerator{}
			}
		}

		err := generator.Generate(&source, out.Directory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating documentation for %s: %v\n", out.Format, err)
			os.Exit(1)
		}
	}
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
func generateCallGraphs(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
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
			if depth >= 2 || visitedCallers[node] {
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

		// Collect callees recursively up to 2 levels
		visitedCallees := make(map[string]bool)
		var calleeEdges []edge
		var collectCallees func(node string, depth int)
		collectCallees = func(node string, depth int) {
			if depth >= 2 || visitedCallees[node] {
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

		// Native PlantUML MindMap representation
		var puml bytes.Buffer
		puml.WriteString("@startmindmap\n")
		puml.WriteString(fmt.Sprintf("* \"%s\"\n", key))

		visitedCallersMap := make(map[string]bool)
		var renderCallers func(node string, depth int)
		renderCallers = func(node string, depth int) {
			if depth >= 5 || visitedCallersMap[node] {
				return
			}
			visitedCallersMap[node] = true
			callers := source.GetCallers(node)
			for _, caller := range callers {
				prefix := strings.Repeat("-", depth+2)
				puml.WriteString(fmt.Sprintf("%s \"%s\"\n", prefix, caller))
				renderCallers(caller, depth+1)
			}
		}
		renderCallers(key, 0)

		visitedCalleesMap := make(map[string]bool)
		var renderCallees func(node string, depth int)
		renderCallees = func(node string, depth int) {
			if depth >= 5 || visitedCalleesMap[node] {
				return
			}
			visitedCalleesMap[node] = true
			callees := source.GetCallees(node)
			for _, callee := range callees {
				prefix := strings.Repeat("+", depth+2)
				puml.WriteString(fmt.Sprintf("%s \"%s\"\n", prefix, callee))
				renderCallees(callee, depth+1)
			}
		}
		renderCallees(key, 0)

		puml.WriteString("@endmindmap\n")

		// Create unique closures to capture key and cleanKey properly
		func(k, ck, dContent, pContent string) {
			*jobs = append(*jobs, DiagramJob{
				DotContent:  dContent,
				PumlContent: pContent,
				PngPath:     pngPath,
				PostRun: func() {
					writeGraphHTMLToAll(fmt.Sprintf("%s_call.html", ck), fmt.Sprintf("%s Call Graph", k), fmt.Sprintf("../images/%s_call_graph.png", ck), outputDirs)
				},
			})
		}(key, cleanKey, dot.String(), puml.String())
	}
}

// generateImportGraph compiles a visual package/file import relationship graph.
func generateImportGraph(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
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

	var puml bytes.Buffer
	puml.WriteString("@startuml\n")
	for _, imp := range imports {
		fileBase := filepath.Base(imp.File)
		puml.WriteString(fmt.Sprintf("[%s] --> [%s]\n", fileBase, imp.Name))
	}
	puml.WriteString("@enduml\n")

	*jobs = append(*jobs, DiagramJob{
		DotContent:  dot.String(),
		PumlContent: puml.String(),
		PngPath:     pngPath,
		PostRun: func() {
			writeGraphHTMLToAll("imports.html", "Import Dependency Graph", "../images/imports_graph.png", outputDirs)
		},
	})
}

// generateFullProgramGraph compiles a visual callee graph of the entire program starting from main.
func generateFullProgramGraph(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
	_ = os.MkdirAll(imagesDir, 0755)

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

	var puml bytes.Buffer
	puml.WriteString("@startmindmap\n")
	puml.WriteString("* \"main\"\n")

	visitedMap := make(map[string]bool)
	var renderMap func(node string, depth int)
	renderMap = func(node string, depth int) {
		if visitedMap[node] {
			return
		}
		visitedMap[node] = true
		callees := source.GetCallees(node)
		for _, callee := range callees {
			prefix := strings.Repeat("+", depth+1)
			puml.WriteString(fmt.Sprintf("%s \"%s\"\n", prefix, callee))
			renderMap(callee, depth+1)
		}
	}
	renderMap("main", 1)

	puml.WriteString("@endmindmap\n")

	*jobs = append(*jobs, DiagramJob{
		DotContent:  dot.String(),
		PumlContent: puml.String(),
		PngPath:     pngPath,
		PostRun: func() {
			writeGraphHTMLToAll("program.html", "Full Program Callee Graph", "../images/program_graph.png", outputDirs)
		},
	})
}

// generateRelationsGraph compiles a visual type relationship diagram (implements, embeds, composition).
func generateRelationsGraph(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
	_ = os.MkdirAll(imagesDir, 0755)

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

		// Inheritance / Explicit Relations
		for _, relName := range s.Relations {
			rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=onormal, style=solid, color=\"#6366F1\", label=\"extends\"];\n", s.Name, relName)
			if !renderedRelations[rel] {
				renderedRelations[rel] = true
				dot.WriteString(rel)
			}
		}

		// Embedding / Composition Relationships
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

	// Native PlantUML Class Diagram
	var puml bytes.Buffer
	puml.WriteString("@startuml\n")
	puml.WriteString("skinparam classAttributeIconSize 0\n")
	for _, it := range interfaces {
		puml.WriteString(fmt.Sprintf("interface %s {\n", it.Name))
		puml.WriteString("}\n")
	}
	for _, s := range structs {
		puml.WriteString(fmt.Sprintf("class %s {\n", s.Name))
		fields := source.GetStructFields(s.Name)
		for _, f := range fields {
			cleanType := strings.ReplaceAll(f.Type, "{", "[")
			cleanType = strings.ReplaceAll(cleanType, "}", "]")
			puml.WriteString(fmt.Sprintf("    +%s : %s\n", f.Name, cleanType))
		}
		methods := source.GetStructMethods(s.Name)
		for _, m := range methods {
			puml.WriteString(fmt.Sprintf("    +%s()\n", m.Name))
		}
		puml.WriteString("}\n")
	}

	renderedPumlRelations := make(map[string]bool)
	for _, s := range structs {
		fields := source.GetStructFields(s.Name)
		methods := source.GetStructMethods(s.Name)

		for _, m := range methods {
			if m.Name == "Parse" {
				rel := fmt.Sprintf("%s ..|> Parser : implements\n", s.Name)
				if !renderedPumlRelations[rel] {
					renderedPumlRelations[rel] = true
					puml.WriteString(rel)
				}
			}
			if m.Name == "Generate" {
				rel := fmt.Sprintf("%s ..|> Generator : implements\n", s.Name)
				if !renderedPumlRelations[rel] {
					renderedPumlRelations[rel] = true
					puml.WriteString(rel)
				}
			}
		}

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
					rel := fmt.Sprintf("%s *-- %s : embeds\n", s.Name, cleanType)
					if !renderedPumlRelations[rel] {
						renderedPumlRelations[rel] = true
						puml.WriteString(rel)
					}
				} else {
					rel := fmt.Sprintf("%s o-- %s : composed of\n", s.Name, cleanType)
					if !renderedPumlRelations[rel] {
						renderedPumlRelations[rel] = true
						puml.WriteString(rel)
					}
				}
			}
		}

		// Inheritance / Explicit Relations
		for _, relName := range s.Relations {
			rel := fmt.Sprintf("%s --|> %s : extends\n", s.Name, relName)
			if !renderedPumlRelations[rel] {
				renderedPumlRelations[rel] = true
				puml.WriteString(rel)
			}
		}
	}
	puml.WriteString("@enduml\n")

	*jobs = append(*jobs, DiagramJob{
		DotContent:  dot.String(),
		PumlContent: puml.String(),
		PngPath:     pngPath,
		PostRun: func() {
			writeGraphHTMLToAll("relations.html", "Type Relationships Graph", "../images/relations_graph.png", outputDirs)
		},
	})
}

// generateSequenceDiagrams compiles a visual sequence diagram for each function showing its call interactions.
func generateSequenceDiagrams(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
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

	numGenerated := 0
	for _, key := range keys {
		if numGenerated >= 15 {
			break
		}
		level1Callers := source.GetCallers(key)
		level1Callees := source.GetCallees(key)

		if len(level1Callers) == 0 && len(level1Callees) == 0 {
			continue
		}
		numGenerated++

		// Collect Level 2 callers
		level2Callers := make(map[string][]string)
		for _, l1 := range level1Callers {
			level2Callers[l1] = source.GetCallers(l1)
		}

		// Collect Level 2 callees
		level2Callees := make(map[string][]string)
		for _, l1 := range level1Callees {
			level2Callees[l1] = source.GetCallees(l1)
		}

		cleanKey := strings.ReplaceAll(key, ".", "_")
		pngPath := filepath.Join(imagesDir, fmt.Sprintf("%s_sequence.png", cleanKey))

		aliases := make(map[string]string)
		aliasIndex := 0
		getAlias := func(name string) string {
			if alias, ok := aliases[name]; ok {
				return alias
			}
			aliasIndex++
			alias := fmt.Sprintf("P%d", aliasIndex)
			aliases[name] = alias
			return alias
		}

		focusAlias := getAlias(key)

		// Register Level 1 and 2 callers/callees aliases
		for _, l1 := range level1Callers {
			_ = getAlias(l1)
			for _, l2 := range level2Callers[l1] {
				_ = getAlias(l2)
			}
		}
		for _, l1 := range level1Callees {
			_ = getAlias(l1)
			for _, l2 := range level2Callees[l1] {
				_ = getAlias(l2)
			}
		}

		var puml bytes.Buffer
		puml.WriteString("@startuml\n")
		puml.WriteString("skinparam ParticipantPadding 10\n")
		puml.WriteString("skinparam BoxPadding 10\n\n")

		puml.WriteString(fmt.Sprintf("participant \"%s\" as %s\n", key, focusAlias))

		var orderedAliases []string
		for name, alias := range aliases {
			if alias != focusAlias {
				orderedAliases = append(orderedAliases, name)
			}
		}
		sort.Strings(orderedAliases)
		for _, name := range orderedAliases {
			puml.WriteString(fmt.Sprintf("participant \"%s\" as %s\n", name, aliases[name]))
		}
		puml.WriteString("\n")

		// Render Level 2 -> Level 1 -> Focus calls
		for _, l1 := range level1Callers {
			l1Alias := getAlias(l1)
			l2List := level2Callers[l1]
			if len(l2List) > 0 {
				for _, l2 := range l2List {
					l2Alias := getAlias(l2)
					puml.WriteString(fmt.Sprintf("%s -> %s : calls\n", l2Alias, l1Alias))
				}
			}
			puml.WriteString(fmt.Sprintf("%s -> %s : calls\n", l1Alias, focusAlias))
		}

		puml.WriteString(fmt.Sprintf("activate %s\n", focusAlias))

		// Render Focus -> Level 1 -> Level 2 callee flows
		for _, l1 := range level1Callees {
			l1Alias := getAlias(l1)
			puml.WriteString(fmt.Sprintf("%s -> %s : calls\n", focusAlias, l1Alias))
			puml.WriteString(fmt.Sprintf("activate %s\n", l1Alias))

			l2List := level2Callees[l1]
			for _, l2 := range l2List {
				l2Alias := getAlias(l2)
				puml.WriteString(fmt.Sprintf("%s -> %s : calls\n", l1Alias, l2Alias))
				puml.WriteString(fmt.Sprintf("activate %s\n", l2Alias))
				puml.WriteString(fmt.Sprintf("%s --> %s : return\n", l2Alias, l1Alias))
				puml.WriteString(fmt.Sprintf("deactivate %s\n", l2Alias))
			}

			puml.WriteString(fmt.Sprintf("%s --> %s : return\n", l1Alias, focusAlias))
			puml.WriteString(fmt.Sprintf("deactivate %s\n", l1Alias))
		}

		puml.WriteString(fmt.Sprintf("deactivate %s\n", focusAlias))
		puml.WriteString("@enduml\n")

		func(k, ck, pContent string) {
			*jobs = append(*jobs, DiagramJob{
				DotContent:  "",
				PumlContent: pContent,
				PngPath:     pngPath,
				PostRun: func() {
					writeGraphHTMLToAll(fmt.Sprintf("%s_sequence.html", ck), fmt.Sprintf("%s Sequence Diagram", k), fmt.Sprintf("../images/%s_sequence.png", ck), outputDirs)
				},
			})
		}(key, cleanKey, puml.String())
	}
}

// generateTimingDiagrams compiles robust timing diagrams representing the lifecycle of structs in program context.
func generateTimingDiagrams(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
	_ = os.MkdirAll(imagesDir, 0755)

	var structs []store.Symbol
	for _, sym := range source.Symbols {
		if sym.Kind == store.SymStruct {
			structs = append(structs, sym)
		}
	}

	numGenerated := 0
	for _, s := range structs {
		if numGenerated >= 15 {
			break
		}
		methods := source.GetStructMethods(s.Name)
		hasStateChange := false
		for _, m := range methods {
			name := strings.ToLower(m.Name)
			if strings.Contains(name, "init") || strings.Contains(name, "parse") || strings.Contains(name, "generate") || strings.Contains(name, "run") || strings.Contains(name, "close") || strings.Contains(name, "stop") {
				hasStateChange = true
				break
			}
		}

		if !hasStateChange {
			continue
		}
		numGenerated++

		// 1. Find constructors for this struct
		var constructorName string
		var creatorName string = "main" // default creator context
		
		for _, sym := range source.Symbols {
			if sym.Kind == store.SymFunction {
				nameLower := strings.ToLower(sym.Name)
				structLower := strings.ToLower(s.Name)
				if strings.Contains(nameLower, "new") && strings.Contains(nameLower, structLower) {
					constructorName = sym.Name
					break
				}
			}
		}

		if constructorName != "" {
			callers := source.GetCallers(constructorName)
			if len(callers) > 0 {
				creatorName = callers[0]
			}
		} else {
			constructorName = fmt.Sprintf("New%s()", s.Name)
		}

		pngPath := filepath.Join(imagesDir, fmt.Sprintf("%s_timing.png", s.Name))

		var puml bytes.Buffer
		puml.WriteString("@startuml\n")
		puml.WriteString("robust \"Program Execution\" as PE\n")
		puml.WriteString(fmt.Sprintf("robust \"Struct '%s' Lifecycle\" as SL\n\n", s.Name))

		puml.WriteString("@0\n")
		puml.WriteString("PE is Running : main()\n")
		puml.WriteString("SL is Uninitialized\n")

		time := 1
		puml.WriteString(fmt.Sprintf("\n@%d\n", time))
		puml.WriteString(fmt.Sprintf("PE is Creating : %s()\n", creatorName))
		puml.WriteString(fmt.Sprintf("SL is Instantiated : via %s()\n", constructorName))

		time += 2
		for _, m := range methods {
			puml.WriteString(fmt.Sprintf("\n@%d\n", time))
			puml.WriteString(fmt.Sprintf("PE is Running : %s()\n", creatorName))
			puml.WriteString(fmt.Sprintf("SL is Active : %s()\n", m.Name))
			time += 2
		}

		puml.WriteString(fmt.Sprintf("\n@%d\n", time))
		puml.WriteString("PE is Running : main()\n")
		puml.WriteString("SL is Terminated : garbage collected\n")
		puml.WriteString("@enduml\n")

		func(structName, pContent string) {
			*jobs = append(*jobs, DiagramJob{
				DotContent:  "",
				PumlContent: pContent,
				PngPath:     pngPath,
				PostRun: func() {
					writeGraphHTMLToAll(fmt.Sprintf("%s_timing.html", structName), fmt.Sprintf("%s Lifecycle Timing", structName), fmt.Sprintf("../images/%s_timing.png", structName), outputDirs)
				},
			})
		}(s.Name, puml.String())
	}
}

// writeGraphHTML creates a premium standalone HTML wrapper for displaying a large Graphviz visualization.
func writeGraphHTMLToAll(filename, title, imageRelPath string, outputDirs []string) {
	for _, outDir := range outputDirs {
		graphsDir := filepath.Join(outDir, "graphs")
		_ = os.MkdirAll(graphsDir, 0755)
		_ = writeGraphHTML(filepath.Join(graphsDir, filename), title, imageRelPath)
	}
}

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
func generateTypeGraphs(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
	_ = os.MkdirAll(imagesDir, 0755)

	var structs []store.Symbol
	var interfaces []store.Symbol
	for _, sym := range source.Symbols {
		if sym.Kind == store.SymStruct {
			structs = append(structs, sym)
		} else if sym.Kind == store.SymInterface {
			interfaces = append(interfaces, sym)
		}
	}

	numGenerated := 0
	for _, s := range structs {
		if numGenerated >= 15 {
			break
		}
		sCopy := s // Capture loop variable
		pngPath := filepath.Join(imagesDir, fmt.Sprintf("%s_type_graph.png", sCopy.Name))
		htmlName := fmt.Sprintf("%s_type.html", sCopy.Name)
		numGenerated++

		var dot bytes.Buffer
		dot.WriteString("digraph G {\n")
		dot.WriteString("    rankdir=BT;\n")
		dot.WriteString("    node [shape=box, style=\"filled,rounded\", color=\"#818CF8\", fontname=\"Helvetica\", fillcolor=\"#EEF2FF\", fontcolor=\"#312E81\", fontsize=10];\n")
		dot.WriteString("    edge [fontname=\"Helvetica\", fontsize=9];\n")

		// Highlight focal struct node
		dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#818CF8\", fontcolor=\"white\", style=\"filled,rounded,bold\"];\n", sCopy.Name))

		fields := source.GetStructFields(sCopy.Name)
		methods := source.GetStructMethods(sCopy.Name)

		renderedRelations := make(map[string]bool)

		// Implements
		for _, m := range methods {
			if m.Name == "Parse" {
				dot.WriteString("    \"Parser\" [fillcolor=\"#ECFDF5\", color=\"#10B981\", fontcolor=\"#064E3B\", style=\"filled,rounded,dashed\"];\n")
				rel := fmt.Sprintf("    \"%s\" -> \"Parser\" [arrowhead=onormal, style=dashed, color=\"#10B981\", label=\"implements\"];\n", sCopy.Name)
				if !renderedRelations[rel] {
					renderedRelations[rel] = true
					dot.WriteString(rel)
				}
			}
			if m.Name == "Generate" {
				dot.WriteString("    \"Generator\" [fillcolor=\"#ECFDF5\", color=\"#10B981\", fontcolor=\"#064E3B\", style=\"filled,rounded,dashed\"];\n")
				rel := fmt.Sprintf("    \"%s\" -> \"Generator\" [arrowhead=onormal, style=dashed, color=\"#10B981\", label=\"implements\"];\n", sCopy.Name)
				if !renderedRelations[rel] {
					renderedRelations[rel] = true
					dot.WriteString(rel)
				}
			}
		}

		// Inheritance / Explicit Relations
		for _, relName := range sCopy.Relations {
			dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#F5F3FF\", color=\"#6366F1\", fontcolor=\"#1E1B4B\", style=\"filled,rounded\"];\n", relName))
			rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=onormal, style=solid, color=\"#6366F1\", label=\"extends\"];\n", sCopy.Name, relName)
			if !renderedRelations[rel] {
				renderedRelations[rel] = true
				dot.WriteString(rel)
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
					rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=empty, style=solid, color=\"#818CF8\", label=\"embeds\"];\n", sCopy.Name, cleanType)
					if !renderedRelations[rel] {
						renderedRelations[rel] = true
						dot.WriteString(rel)
					}
				} else {
					rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=diamond, style=solid, color=\"#F59E0B\", label=\"composed of\"];\n", sCopy.Name, cleanType)
					if !renderedRelations[rel] {
						renderedRelations[rel] = true
						dot.WriteString(rel)
					}
				}
			}
		}

		dot.WriteString("}\n")

		// Native PlantUML Class Diagram representation
		var puml bytes.Buffer
		puml.WriteString("@startuml\n")
		puml.WriteString("skinparam classAttributeIconSize 0\n")
		puml.WriteString(fmt.Sprintf("class %s {\n", sCopy.Name))
		for _, f := range fields {
			cleanType := strings.ReplaceAll(f.Type, "{", "[")
			cleanType = strings.ReplaceAll(cleanType, "}", "]")
			puml.WriteString(fmt.Sprintf("    +%s : %s\n", f.Name, cleanType))
		}
		for _, m := range methods {
			puml.WriteString(fmt.Sprintf("    +%s()\n", m.Name))
		}
		puml.WriteString("}\n")

		renderedPumlRelations := make(map[string]bool)
		for _, m := range methods {
			if m.Name == "Parse" {
				puml.WriteString("interface Parser\n")
				rel := fmt.Sprintf("%s ..|> Parser : implements\n", sCopy.Name)
				if !renderedPumlRelations[rel] {
					renderedPumlRelations[rel] = true
					puml.WriteString(rel)
				}
			}
			if m.Name == "Generate" {
				puml.WriteString("interface Generator\n")
				rel := fmt.Sprintf("%s ..|> Generator : implements\n", sCopy.Name)
				if !renderedPumlRelations[rel] {
					renderedPumlRelations[rel] = true
					puml.WriteString(rel)
				}
			}
		}

		// Inheritance / Explicit Relations
		for _, relName := range sCopy.Relations {
			puml.WriteString(fmt.Sprintf("class %s\n", relName))
			rel := fmt.Sprintf("%s --|> %s : extends\n", sCopy.Name, relName)
			if !renderedPumlRelations[rel] {
				renderedPumlRelations[rel] = true
				puml.WriteString(rel)
			}
		}

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
				puml.WriteString(fmt.Sprintf("class %s\n", cleanType))
				if f.Name == f.Type || strings.HasSuffix(f.Type, f.Name) {
					rel := fmt.Sprintf("%s *-- %s : embeds\n", sCopy.Name, cleanType)
					if !renderedPumlRelations[rel] {
						renderedPumlRelations[rel] = true
						puml.WriteString(rel)
					}
				} else {
					rel := fmt.Sprintf("%s o-- %s : composed of\n", sCopy.Name, cleanType)
					if !renderedPumlRelations[rel] {
						renderedPumlRelations[rel] = true
						puml.WriteString(rel)
					}
				}
			}
		}
		puml.WriteString("@enduml\n")

		*jobs = append(*jobs, DiagramJob{
			DotContent:  dot.String(),
			PumlContent: puml.String(),
			PngPath:     pngPath,
			PostRun: func() {
				writeGraphHTMLToAll(htmlName, fmt.Sprintf("%s Type Relationship Graph", sCopy.Name), fmt.Sprintf("../images/%s_type_graph.png", sCopy.Name), outputDirs)
			},
		})
	}

	for _, s := range interfaces {
		sCopy := s // Capture loop variable
		pngPath := filepath.Join(imagesDir, fmt.Sprintf("%s_type_graph.png", sCopy.Name))
		htmlName := fmt.Sprintf("%s_type.html", sCopy.Name)

		var dot bytes.Buffer
		dot.WriteString("digraph G {\n")
		dot.WriteString("    rankdir=BT;\n")
		dot.WriteString("    node [shape=box, style=\"filled,rounded\", color=\"#818CF8\", fontname=\"Helvetica\", fillcolor=\"#EEF2FF\", fontcolor=\"#312E81\", fontsize=10];\n")
		dot.WriteString("    edge [fontname=\"Helvetica\", fontsize=9];\n")

		// Highlight focal interface node
		dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#ECFDF5\", color=\"#10B981\", fontcolor=\"#064E3B\", style=\"filled,rounded,dashed,bold\"];\n", sCopy.Name))

		renderedRelations := make(map[string]bool)
		
		// Inheritance / Explicit Relations for Interface
		for _, relName := range sCopy.Relations {
			dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#F5F3FF\", color=\"#6366F1\", fontcolor=\"#1E1B4B\", style=\"filled,rounded\"];\n", relName))
			rel := fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=onormal, style=solid, color=\"#6366F1\", label=\"extends\"];\n", sCopy.Name, relName)
			if !renderedRelations[rel] {
				renderedRelations[rel] = true
				dot.WriteString(rel)
			}
		}

		// Find structs that implement this interface
		for _, other := range structs {
			methods := source.GetStructMethods(other.Name)
			for _, m := range methods {
				if (sCopy.Name == "Parser" && m.Name == "Parse") || (sCopy.Name == "Generator" && m.Name == "Generate") {
					dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#EEF2FF\", color=\"#818CF8\", fontcolor=\"#312E81\", style=\"filled,rounded\"];\n", other.Name))
					dot.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\" [arrowhead=onormal, style=dashed, color=\"#10B981\", label=\"implements\"];\n", other.Name, sCopy.Name))
				}
			}
		}

		dot.WriteString("}\n")

		// Native PlantUML representation for Interface
		var puml bytes.Buffer
		puml.WriteString("@startuml\n")
		puml.WriteString("skinparam classAttributeIconSize 0\n")
		puml.WriteString(fmt.Sprintf("interface %s\n", sCopy.Name))
		
		// Inheritance / Explicit Relations for Interface
		for _, relName := range sCopy.Relations {
			puml.WriteString(fmt.Sprintf("interface %s\n", relName))
			puml.WriteString(fmt.Sprintf("%s --|> %s : extends\n", sCopy.Name, relName))
		}
		for _, other := range structs {
			methods := source.GetStructMethods(other.Name)
			for _, m := range methods {
				if (sCopy.Name == "Parser" && m.Name == "Parse") || (sCopy.Name == "Generator" && m.Name == "Generate") {
					puml.WriteString(fmt.Sprintf("class %s\n", other.Name))
					puml.WriteString(fmt.Sprintf("%s ..|> %s : implements\n", other.Name, sCopy.Name))
				}
			}
		}
		puml.WriteString("@enduml\n")

		*jobs = append(*jobs, DiagramJob{
			DotContent:  dot.String(),
			PumlContent: puml.String(),
			PngPath:     pngPath,
			PostRun: func() {
				copyImageToAll(pngPath, filepath.Base(pngPath), outputDirs)
				writeGraphHTMLToAll(htmlName, fmt.Sprintf("%s Interface Implementations", sCopy.Name), fmt.Sprintf("../images/%s_type_graph.png", sCopy.Name), outputDirs)
			},
		})
	}
}

func copyImageToAll(srcPng string, filename string, outputDirs []string) {
	if len(outputDirs) <= 1 {
		return
	}
	data, err := os.ReadFile(srcPng)
	if err != nil {
		return
	}
	for i := 1; i < len(outputDirs); i++ {
		outImgDir := filepath.Join(outputDirs[i], "images")
		_ = os.MkdirAll(outImgDir, 0755)
		_ = os.WriteFile(filepath.Join(outImgDir, filename), data, 0644)
	}
}

func generatePatternGraphs(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 || len(source.Patterns) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
	_ = os.MkdirAll(imagesDir, 0755)

	for i, p := range source.Patterns {
		if len(p.Symbols) == 0 {
			continue
		}
		// Generate simple hash identifier for the pattern
		patternID := fmt.Sprintf("pattern_%d", i)
		pngPath := filepath.Join(imagesDir, patternID+".png")
		htmlName := patternID + "_graph.html"

		var dot bytes.Buffer
		dot.WriteString("digraph G {\n")
		dot.WriteString("    rankdir=TB;\n")
		dot.WriteString("    node [shape=record, style=\"filled,rounded\", fontname=\"Helvetica\", fillcolor=\"#F0F4F8\", color=\"#4A90E2\"];\n")
		dot.WriteString("    edge [color=\"#999999\", fontname=\"Helvetica\", fontsize=10];\n")
		dot.WriteString(fmt.Sprintf("    label=\"Design Pattern: %s\\nCategory: %s\";\n", p.Name, p.Category))
		dot.WriteString("    labelloc=\"t\";\n")

		// Create nodes for symbols
		for _, sym := range p.Symbols {
			dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#D6EAF8\", style=\"filled,bold\"];\n", sym))
		}
		// Draw connections between sequential members of the pattern cluster
		for j := 0; j < len(p.Symbols)-1; j++ {
			dot.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\" [label=\"participates\"];\n", p.Symbols[j], p.Symbols[j+1]))
		}
		dot.WriteString("}\n")

		var puml bytes.Buffer
		puml.WriteString("@startuml\n")
		puml.WriteString(fmt.Sprintf("title \"Design Pattern: %s (%s)\"\n", p.Name, p.Category))
		puml.WriteString("skinparam classBackgroundColor #F4F7F6\n")
		puml.WriteString("skinparam classBorderColor #2C3E50\n")

		safeCat := strings.ReplaceAll(p.Category, " ", "")
		safeCat = strings.ReplaceAll(safeCat, "/", "")

		for _, sym := range p.Symbols {
			puml.WriteString(fmt.Sprintf("class \"%s\" << %s >>\n", sym, safeCat))
		}
		for j := 0; j < len(p.Symbols)-1; j++ {
			puml.WriteString(fmt.Sprintf("\"%s\" -- \"%s\" : relates\n", p.Symbols[j], p.Symbols[j+1]))
		}
		puml.WriteString("@enduml\n")

		*jobs = append(*jobs, DiagramJob{
			DotContent:  dot.String(),
			PumlContent: puml.String(),
			PngPath:     pngPath,
			PostRun: func() {
				copyImageToAll(pngPath, filepath.Base(pngPath), outputDirs)
				writeGraphHTMLToAll(htmlName, fmt.Sprintf("%s Pattern Visualization", p.Name), fmt.Sprintf("../images/%s.png", patternID), outputDirs)
			},
		})
	}
}

func generateNetworkGraphs(source *store.Source, outputDirs []string, jobs *[]DiagramJob) {
	if len(outputDirs) == 0 || len(source.NetworkAnalysis) == 0 {
		return
	}
	imagesDir := filepath.Join(outputDirs[0], "images")
	_ = os.MkdirAll(imagesDir, 0755)

	for i, nc := range source.NetworkAnalysis {
		if len(nc.Symbols) == 0 {
			continue
		}
		id := fmt.Sprintf("network_%d", i)
		pngPath := filepath.Join(imagesDir, id+".png")
		htmlName := id + "_graph.html"

		var dot bytes.Buffer
		dot.WriteString("digraph G {\n")
		dot.WriteString("    rankdir=LR;\n") 
		dot.WriteString("    node [shape=box, style=\"filled,rounded\", fontname=\"Helvetica\", fillcolor=\"#EAF2F8\", color=\"#2E86C1\"];\n")
		dot.WriteString("    edge [color=\"#34495E\", fontname=\"Helvetica\", fontsize=10, style=dashed];\n")
		dot.WriteString(fmt.Sprintf("    label=\"Network Architecture: %s\\nType: %s\";\n", nc.Name, nc.Type))
		dot.WriteString("    labelloc=\"t\";\n")

		for _, sym := range nc.Symbols {
			dot.WriteString(fmt.Sprintf("    \"%s\" [fillcolor=\"#D4E6F1\", style=\"filled,bold\", shape=ellipse];\n", sym))
		}
		dot.WriteString("}\n")

		var puml bytes.Buffer
		puml.WriteString("@startuml\n")
		puml.WriteString(fmt.Sprintf("title \"Network Component: %s (%s)\"\n", nc.Name, nc.Type))
		puml.WriteString("skinparam componentStyle uml2\n")
		puml.WriteString("skinparam packageBackgroundColor #FFFFFF\n")
		puml.WriteString("skinparam interfaceBackgroundColor #FEFECE\n")
		
		safeType := strings.ReplaceAll(nc.Type, " ", "")
		puml.WriteString(fmt.Sprintf("package \"%s\" <<%s>> {\n", nc.Name, safeType))
		for _, sym := range nc.Symbols {
			puml.WriteString(fmt.Sprintf("  [ %s ]\n", sym))
		}
		puml.WriteString("}\n")
		puml.WriteString("@enduml\n")

		*jobs = append(*jobs, DiagramJob{
			DotContent:  dot.String(),
			PumlContent: puml.String(),
			PngPath:     pngPath,
			PostRun: func() {
				copyImageToAll(pngPath, filepath.Base(pngPath), outputDirs)
				writeGraphHTMLToAll(htmlName, fmt.Sprintf("%s Network Visualization", nc.Name), fmt.Sprintf("../images/%s.png", id), outputDirs)
			},
		})
	}
}
