import re
with open("cmd/generate/main.go", "r") as f:
    text = f.read()

# Replace signatures
text = re.sub(r'func generateCallGraphs\(source \*store\.Source\) \{', 'func generateCallGraphs(source *store.Source, outputDirs []string) {', text)
text = re.sub(r'func generateImportGraph\(source \*store\.Source\) \{', 'func generateImportGraph(source *store.Source, outputDirs []string) {', text)
text = re.sub(r'func generateFullProgramGraph\(source \*store\.Source\) \{', 'func generateFullProgramGraph(source *store.Source, outputDirs []string) {', text)
text = re.sub(r'func generateRelationsGraph\(source \*store\.Source\) \{', 'func generateRelationsGraph(source *store.Source, outputDirs []string) {', text)
text = re.sub(r'func generateTypeGraphs\(source \*store\.Source\) \{', 'func generateTypeGraphs(source *store.Source, outputDirs []string) {', text)

# Replace imagesDir
text = text.replace('imagesDir := filepath.Join("docs", "images")', '''if len(outputDirs) == 0 { return }
	imagesDir := filepath.Join(outputDirs[0], "images")''')

# Add copy helper
text += """
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

# Replace all exec.Command instances that output PNG
# e.g.: exec.Command("dot", "-Tpng", dotPath, "-o", pngPath).Run()
text = re.sub(r'(exec\.Command\("dot", "-Tpng", [^,]+, "-o", ([^)]+)\)\.Run\(\))', r'\1\n\t\tcopyImageToAll(\2, filepath.Base(\2), outputDirs)', text)

with open("cmd/generate/main.go", "w") as f:
    f.write(text)
