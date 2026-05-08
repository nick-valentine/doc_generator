import re

with open('cmd/generate/main.go', 'r') as f:
    content = f.read()

# Add toml to imports
content = re.sub(r'import \(\n', 'import (\n\t"github.com/pelletier/go-toml/v2"\n', content, count=1)

# Add Config struct
config_struct = """
type Config struct {
	Input struct {
		Directory string   `toml:"directory"`
		Ignore    []string `toml:"ignore"`
	} `toml:"input"`
	Output []struct {
		Format    string `toml:"format"`
		Directory string `toml:"directory"`
	} `toml:"output"`
}
"""
content = re.sub(r'func main\(\) \{', config_struct + '\nfunc main() {', content, count=1)

# Replace main body
old_main_body = r'''	audienceFlag := flag\.String\("audience".*?os\.Exit\(1\)\n	\}'''
new_main_body = """
	configFlag := flag.String("config", "docgen.toml", "Path to TOML configuration file")
	audienceFlag := flag.String("audience", "", "Filter symbols by audience (e.g. API, INTERNAL, USER, DEVELOPER)")
	flag.Parse()

	configData, err := os.ReadFile(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config file %s: %v\\n", *configFlag, err)
		os.Exit(1)
	}

	var config Config
	if err := toml.Unmarshal(configData, &config); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config file: %v\\n", err)
		os.Exit(1)
	}

	if config.Input.Directory == "" {
		fmt.Fprintf(os.Stderr, "Error: input.directory must be specified in config\\n")
		os.Exit(1)
	}

	source := store.Source{}

	parsersList, err := loadParserPlugins()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading parser plugins: %v\\n", err)
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
		fmt.Fprintf(os.Stderr, "Error scanning files: %v\\n", err)
		os.Exit(1)
	}

	if *audienceFlag != "" {
		filteredSymbols := source.FilterByAudience(*audienceFlag)
		source.Symbols = filteredSymbols
	}

	var outputDirs []string
	for _, out := range config.Output {
		outputDirs = append(outputDirs, out.Directory)
		_ = os.MkdirAll(out.Directory, 0755)
	}

	// Generate the static call graph and import graph images
	generateCallGraphs(&source, outputDirs)
	generateImportGraph(&source, outputDirs)
	generateFullProgramGraph(&source, outputDirs)
	generateRelationsGraph(&source, outputDirs)
	generateTypeGraphs(&source, outputDirs)

	generatorsList, err := loadGeneratorPlugins()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading generator plugins: %v\\n", err)
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
			fmt.Fprintf(os.Stderr, "Error generating documentation for %s: %v\\n", out.Format, err)
		} else {
			fmt.Printf("Generated %s documentation in %s\\n", out.Format, out.Directory)
		}
	}
}

func DUMMY() {
"""

content = re.sub(r'\targs := flag\.Args\(\)[\s\S]*?\tfmt\.Print\(output\)\n\}', new_main_body.strip(), content, count=1)

# Update graph functions signatures
content = re.sub(r'func generateCallGraphs\(source \*store\.Source\) \{', 'func generateCallGraphs(source *store.Source, outputDirs []string) {', content)
content = re.sub(r'func generateImportGraph\(source \*store\.Source\) \{', 'func generateImportGraph(source *store.Source, outputDirs []string) {', content)
content = re.sub(r'func generateFullProgramGraph\(source \*store\.Source\) \{', 'func generateFullProgramGraph(source *store.Source, outputDirs []string) {', content)
content = re.sub(r'func generateRelationsGraph\(source \*store\.Source\) \{', 'func generateRelationsGraph(source *store.Source, outputDirs []string) {', content)
content = re.sub(r'func generateTypeGraphs\(source \*store\.Source\) \{', 'func generateTypeGraphs(source *store.Source, outputDirs []string) {', content)

# Now, handle the imagesDir logic inside these functions.
# They do: imagesDir := filepath.Join("docs", "images")
# We will change it to:
# 	if len(outputDirs) == 0 { return }
# 	imagesDir := filepath.Join(outputDirs[0], "images")

content = re.sub(r'imagesDir := filepath\.Join\("docs", "images"\)', 
'''	if len(outputDirs) == 0 { return }
	imagesDir := filepath.Join(outputDirs[0], "images")''', content)

# But they also need to copy the generated pngs to the other output directories.
# A helper function `copyImageToAll`
copy_helper = """
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
"""

content = content + "\n" + copy_helper

# Find where dot is executed and add copyImageToAll
# Example: exec.Command("dot", "-Tpng", dotPath, "-o", pngPath).Run()
content = re.sub(r'(exec\.Command\("dot", "-Tpng", [^,]+, "-o", ([^)]+)\)\.Run\(\))', r'\1\n\t\tcopyImageToAll(\2, filepath.Base(\2), outputDirs)', content)

with open('cmd/generate/main.go', 'w') as f:
    f.write(content)

