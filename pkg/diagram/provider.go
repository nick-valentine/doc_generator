package diagram

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type DiagramProvider interface {
	Generate(content string, outputPath string) error
	IsAvailable() bool
	Name() string
}

type DotDiagramProvider struct{}

func (p *DotDiagramProvider) Name() string { return "Graphviz Dot" }

func (p *DotDiagramProvider) IsAvailable() bool {
	_, err := exec.LookPath("dot")
	return err == nil
}

func (p *DotDiagramProvider) Generate(content string, outputPath string) error {
	// Enforce maximum layout rendering timeout to prevent combinatorial explosions from pegging CPU infinitely
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dot", "-Tsvg")
	cmd.Stdin = strings.NewReader(content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("dot execution timed out after 20 seconds")
		}
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

func (p *PlantUMLDiagramProvider) Generate(content string, outputPath string) error {
	// Enforce maximum layout rendering timeout to prevent combinatorial explosions from pegging CPU infinitely
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "plantuml", "-tsvg", "-pipe")
	cmd.Env = append(os.Environ(), "PLANTUML_LIMIT_SIZE=65536")
	
	var input string
	if strings.HasPrefix(strings.TrimSpace(content), "@start") {
		input = content
	} else {
		input = fmt.Sprintf("@startdot\n%s\n@enddot", content)
	}
	cmd.Stdin = strings.NewReader(input)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("plantuml execution timed out after 20 seconds")
		}
		return fmt.Errorf("plantuml execution failed: %v (stderr: %s)", err, stderr.String())
	}
	return nil
}

type SmartDiagramProvider struct{}

func (p *SmartDiagramProvider) Name() string { return "Smart Engine Router" }
func (p *SmartDiagramProvider) IsAvailable() bool { return true }

func (p *SmartDiagramProvider) Generate(content string, outputPath string) error {
	isPuml := strings.Contains(strings.TrimSpace(content), "@start")
	
	if !isPuml {
		// Optimize for speed: try Graphviz first for pure DOT graphs
		d := GetProviderByName("graphviz dot")
		if d != nil {
			return d.Generate(content, outputPath)
		}
	}
	
	// PlantUML or fallback
	var fallback DiagramProvider
	if isPuml {
		fallback = GetProviderByName("plantuml")
	}
	
	if fallback == nil {
		// Fall through to whatever we can find that IS NOT this smart provider to avoid infinite looping
		for _, item := range providers {
			if item.Name() != p.Name() && item.IsAvailable() {
				fallback = item
				break
			}
		}
	}
	
	if fallback != nil {
		return fallback.Generate(content, outputPath)
	}
	
	return fmt.Errorf("no suitable diagram generator found for this content type")
}

var providers []DiagramProvider

func RegisterProvider(p DiagramProvider) {
	providers = append(providers, p)
}

func init() {
	// Register in ascending dependency order: lower-level tools first
	RegisterProvider(&PlantUMLDiagramProvider{})
	RegisterProvider(&DotDiagramProvider{})
	// Finally, register the smart manager that orchestrates the rest
	RegisterProvider(&SmartDiagramProvider{})
}

func GetBestProvider() DiagramProvider {
	// Preference order: Smart Router (Optimizes Mixed Formats) > PlantUML > Graphviz Dot
	order := []string{"smart engine router", "plantuml", "graphviz dot", "go-graphviz"}
	for _, name := range order {
		for _, p := range providers {
			pName := strings.ToLower(p.Name())
			if strings.Contains(pName, name) {
				if p.IsAvailable() {
					return p
				}
			}
		}
	}
	// Fallback to any available provider if none matched
	for _, p := range providers {
		if p.IsAvailable() {
			return p
		}
	}
	return nil
}

func GetProviderByName(name string) DiagramProvider {
	nameLower := strings.ToLower(name)
	for _, p := range providers {
		pName := strings.ToLower(p.Name())
		if strings.Contains(pName, nameLower) || strings.Contains(strings.ReplaceAll(pName, "-", ""), nameLower) {
			if p.IsAvailable() {
				return p
			}
		}
	}
	return nil
}
