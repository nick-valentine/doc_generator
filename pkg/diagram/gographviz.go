package diagram

import (
	"fmt"
	"strings"
	"sync"

	"github.com/goccy/go-graphviz"
)

type GoGraphvizDiagramProvider struct {
	mu sync.Mutex
}

func (p *GoGraphvizDiagramProvider) Name() string { return "Go-Graphviz (Native)" }

func (p *GoGraphvizDiagramProvider) IsAvailable() bool { return true }

func (p *GoGraphvizDiagramProvider) Generate(content string, outputPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	g := graphviz.New()
	defer g.Close()

	var input string
	if strings.HasPrefix(strings.TrimSpace(content), "@start") {
		input = content
		input = strings.TrimPrefix(input, "@startdot")
		input = strings.TrimPrefix(input, "@start")
		input = strings.TrimSuffix(input, "@enddot")
		input = strings.TrimSuffix(input, "@enduml")
		input = strings.TrimSuffix(input, "@end")
		input = strings.TrimSpace(input)
	} else {
		input = content
	}

	graph, err := graphviz.ParseBytes([]byte(input))
	if err != nil {
		return fmt.Errorf("go-graphviz parse failed: %w", err)
	}
	defer graph.Close()

	if err := g.RenderFilename(graph, graphviz.SVG, outputPath); err != nil {
		return fmt.Errorf("go-graphviz render failed: %w", err)
	}
	return nil
}

func init() {
	RegisterProvider(&GoGraphvizDiagramProvider{})
}
