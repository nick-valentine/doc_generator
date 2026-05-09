package diagram

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type DiagramProvider interface {
	Generate(content string, pngPath string) error
	IsAvailable() bool
	Name() string
}

type DotDiagramProvider struct{}

func (p *DotDiagramProvider) Name() string { return "Graphviz Dot" }

func (p *DotDiagramProvider) IsAvailable() bool {
	_, err := exec.LookPath("dot")
	return err == nil
}

func (p *DotDiagramProvider) Generate(content string, pngPath string) error {
	cmd := exec.Command("dot", "-Tpng")
	cmd.Stdin = strings.NewReader(content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	outFile, err := os.Create(pngPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dot execution failed: %v (stderr: %s)", err, stderr.String())
	}
	return nil
}

type PlantUMLDiagramProvider struct{}

func (p *PlantUMLDiagramProvider) Name() string { return "PlantUML" }

func (p *PlantUMLDiagramProvider) IsAvailable() bool {
	_, err := exec.LookPath("plantuml")
	return err == nil
}

func (p *PlantUMLDiagramProvider) Generate(content string, pngPath string) error {
	cmd := exec.Command("plantuml", "-tpng", "-pipe")
	cmd.Env = append(os.Environ(), "PLANTUML_LIMIT_SIZE=32768")
	
	var input string
	if strings.HasPrefix(strings.TrimSpace(content), "@start") {
		input = content
	} else {
		input = fmt.Sprintf("@startdot\n%s\n@enddot", content)
	}
	cmd.Stdin = strings.NewReader(input)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	outFile, err := os.Create(pngPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("plantuml execution failed: %v (stderr: %s)", err, stderr.String())
	}
	return nil
}

func GetBestProvider() DiagramProvider {
	puml := &PlantUMLDiagramProvider{}
	if puml.IsAvailable() {
		return puml
	}
	dot := &DotDiagramProvider{}
	if dot.IsAvailable() {
		return dot
	}
	return nil
}
