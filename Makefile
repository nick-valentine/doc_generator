# Makefile for Self-Documenting Parser with Call Graphs
# A modern Go documentation pipeline that supports multi-language analysis.

# Dynamically resolve the full path to 'go' to prevent cases where a directory named 'go' shadows the binary in PATH
GO          ?= $(shell command -v go 2>/dev/null || which go 2>/dev/null || echo go)
BUILD_FLAGS ?= -buildmode=plugin
CMD_SRC     ?= cmd/generate/main.go

.PHONY: all plugins html text test clean help docs-beholderFPS docs-playtranslate docs-hadoop docs-mattermost docs-skills

# Default target generates the documentation and plugins
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
	@echo "  make docs-mattermost    Generate HTML/MD docs for the Mattermost JS/Go codebase using mattermost.toml"
	@echo "  make docs-skills        Scan internal AI skills directory (~/.gemini/antigravity/skills) for vulnerabilities using skills.toml"
	@echo "  make clean              Remove generated documentation files and built plugins"

plugins:
	@echo "Building dynamic plugins..."
	@mkdir -p plugins/parsers plugins/generators
	$(GO) build $(BUILD_FLAGS) -o plugins/parsers/go_parser.so plugins/parsers/go/main.go
	$(GO) build $(BUILD_FLAGS) -o plugins/parsers/markdown_parser.so plugins/parsers/markdown/main.go
	$(GO) build $(BUILD_FLAGS) -o plugins/generators/html_generator.so plugins/generators/html/main.go
	$(GO) build $(BUILD_FLAGS) -o plugins/generators/text_generator.so plugins/generators/text/main.go
	$(GO) build $(BUILD_FLAGS) -o plugins/parsers/odin_parser.so plugins/parsers/odin/main.go
	$(GO) build $(BUILD_FLAGS) -o plugins/parsers/java_parser.so plugins/parsers/java/main.go
	$(GO) build $(BUILD_FLAGS) -o plugins/parsers/python_parser.so plugins/parsers/python/main.go
	$(GO) build $(BUILD_FLAGS) -o plugins/parsers/kotlin_parser.so plugins/parsers/kotlin/main.go
	$(GO) build $(BUILD_FLAGS) -o plugins/parsers/javascript_parser.so plugins/parsers/javascript/main.go
	@echo "Plugins built successfully."

html: plugins
	@echo "Generating premium HTML and Markdown documentation (self-documentation)..."
	$(GO) run $(CMD_SRC) -config selfdoc.toml
	@echo "Documentation generated in docs/html and docs/markdown"

text: html
	@echo "Text target relies on html target since selfdoc.toml generates both."

test:
	@echo "Running unit tests..."
	$(GO) test -v ./...

docs-beholderFPS: plugins
	@echo "Generating beholderFPS documentation (Odin codebase)..."
	$(GO) run $(CMD_SRC) -config docgen.toml
	@echo "Documentation generated in docs/beholderFPS and docs/beholderFPS_md"

docs-playtranslate: plugins
	@echo "Generating playtranslate documentation (Kotlin/Python codebase)..."
	$(GO) run $(CMD_SRC) -config playtranslate.toml
	@echo "Documentation generated in docs/playtranslate and docs/playtranslate_md"

docs-hadoop: plugins
	@echo "Generating Hadoop documentation (Java codebase)..."
	$(GO) run $(CMD_SRC) -config hadoop.toml
	@echo "Documentation generated in docs/hadoop and docs/hadoop_md"

docs-mattermost: plugins
	@echo "Generating Mattermost documentation (Go/TypeScript codebase)..."
	$(GO) run $(CMD_SRC) -config mattermost.toml
	@echo "Documentation generated in docs/mattermost"

docs-skills: plugins
	@echo "Analyzing Antigravity system skills for AI weakness attack vectors..."
	$(GO) run $(CMD_SRC) -config skills.toml
	@echo "Documentation and vulnerability findings generated in docs/skills"


clean:
	@echo "Cleaning up generated documentation files..."
	rm -rf docs/
	rm -rf plugins/parsers/*.so
	rm -rf plugins/generators/*.so
	@echo "Cleanup complete."
