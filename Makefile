# Makefile for Self-Documenting Go Parser with Call Graphs

.PHONY: all html text test clean help plugins docs-beholderFPS docs-playtranslate docs-hadoop

# Default target generates the premium HTML documentation
all: plugins html

help:
	@echo "Available Makefile targets:"
	@echo "  make plugins            Build all dynamic shared object (.so) plugins"
	@echo "  make html               Generate HTML documentation for this repo using selfdoc.toml"
	@echo "  make text               Generate Markdown self-documentation using selfdoc.toml"
	@echo "  make test               Run all parser, store, and generator unit tests"
	@echo "  make docs-beholderFPS   Generate HTML/MD docs for the beholderFPS Odin codebase using docgen.toml"
	@echo "  make docs-playtranslate Generate HTML/MD docs for the playtranslate Kotlin/Python codebase using playtranslate.toml"
	@echo "  make docs-hadoop        Generate HTML/MD docs for the Apache Hadoop Java codebase using hadoop.toml"
	@echo "  make clean              Remove generated files and plugins"

plugins:
	@echo "Building dynamic plugins..."
	@mkdir -p plugins/parsers plugins/generators
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/parsers/go_parser.so plugins/parsers/go/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/parsers/markdown_parser.so plugins/parsers/markdown/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/generators/html_generator.so plugins/generators/html/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/generators/text_generator.so plugins/generators/text/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/parsers/odin_parser.so plugins/parsers/odin/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/parsers/java_parser.so plugins/parsers/java/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/parsers/python_parser.so plugins/parsers/python/main.go
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go build -buildmode=plugin -o plugins/parsers/kotlin_parser.so plugins/parsers/kotlin/main.go
	@echo "Plugins built successfully."

html: plugins
	@echo "Generating premium HTML and Markdown documentation (self-documentation)..."
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go run cmd/generate/main.go -config selfdoc.toml
	@echo "Documentation generated in docs/html and docs/markdown"

text: html
	@echo "Text target relies on html target since selfdoc.toml generates both."

test:
	@echo "Running unit tests..."
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go test -v ./...

docs-beholderFPS: plugins
	@echo "Generating beholderFPS documentation (Odin codebase)..."
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go run cmd/generate/main.go -config docgen.toml
	@echo "Documentation generated in docs/beholderFPS and docs/beholderFPS_md"

docs-playtranslate: plugins
	@echo "Generating playtranslate documentation (Kotlin/Python codebase)..."
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go run cmd/generate/main.go -config playtranslate.toml
	@echo "Documentation generated in docs/playtranslate and docs/playtranslate_md"

docs-hadoop: plugins
	@echo "Generating Hadoop documentation (Java codebase)..."
	GOMODCACHE=/home/nick/go/pkg/mod GOPATH=$$(pwd)/.gopath go run cmd/generate/main.go -config hadoop.toml
	@echo "Documentation generated in docs/hadoop and docs/hadoop_md"

clean:
	@echo "Cleaning up generated documentation files..."
	rm -rf docs/
	rm -rf plugins
	@echo "Cleanup complete."
