# Makefile for Self-Documenting Go Parser with Call Graphs

.PHONY: all html text test clean help plugins

# Default target generates the premium HTML documentation
all: plugins html

help:
	@echo "Available Makefile targets:"
	@echo "  make plugins   Build all dynamic shared object (.so) plugins"
	@echo "  make html      Generate the premium glassmorphic HTML documentation dashboard"
	@echo "  make text      Generate the Markdown self-documentation file"
	@echo "  make test      Run all parser, store, and generator unit tests"
	@echo "  make clean     Remove generated HTML, Markdown, and static call graph images"

plugins:
	@echo "Building dynamic plugins..."
	@mkdir -p plugins/parsers plugins/generators
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/parsers/go_parser.so plugins/parsers/go/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/parsers/markdown_parser.so plugins/parsers/markdown/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/generators/html_generator.so plugins/generators/html/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/generators/text_generator.so plugins/generators/text/main.go
	@echo "Plugins built successfully."

html: plugins
	@echo "Generating premium HTML documentation..."
	@mkdir -p docs/images
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go run cmd/generate/main.go -format html . > docs/index.html
	@echo "Documentation generated successfully: docs/index.html"

text: plugins
	@echo "Generating Markdown self-documentation..."
	@mkdir -p docs/images
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go run cmd/generate/main.go -format text . > docs/self_documentation.md
	@echo "Documentation generated successfully: docs/self_documentation.md"

test:
	@echo "Running unit tests..."
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go test -v ./...

clean:
	@echo "Cleaning up generated documentation files..."
	rm -f docs/index.html docs/self_documentation.md
	rm -rf docs/images/*_call_graph.png
	rm -rf plugins
	@echo "Cleanup complete."
