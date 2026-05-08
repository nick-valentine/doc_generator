# Dynamic Plugin Architecture

This document outlines the architecture and implementation of the dynamic plugin-based self-documenting parser system.

## Overview

The `doc_generator` utility is now built around a **Dynamic Plugin Architecture** leveraging Go's `plugin` package. Instead of hardcoding parsers and generators in the main host binary, they are built as dynamically loadable shared objects (`.so`) that can be placed in standard directories:
- `plugins/parsers/` — Dynamic shared object parser plugins
- `plugins/generators/` — Dynamic shared object generator plugins

## Component Breakdown

1. **Host Application** (`cmd/generate/main.go`):
   - Scans the directory for `.so` files at runtime.
   - Dynamically loads parsers and maps them to supported file extensions.
   - Dynamically loads generators and selects the appropriate one based on `-format` flag.
2. **Go Parser Plugin** (`plugins/parsers/go_parser.so`):
   - Handles `.go` extensions.
   - Extracts AST nodes via Tree-Sitter.
3. **Markdown Parser Plugin** (`plugins/parsers/markdown_parser.so`):
   - Handles `.md` extensions.
   - Parses markdown documents into raw contents.

## Features

- **User Extensible**: Developers can easily add custom parsers for other languages (e.g. Python, JS) without modifying the host core.
- **Compiled Markdown Output**: In-tree markdown files render in the final HTML dashboard with high-fidelity formatting, rich CSS headings, custom bullet lists, and code highlighting!
