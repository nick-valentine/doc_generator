module doc_generator/plugins/parsers/kotlin

go 1.23.0

require (
	doc_generator v0.0.0
	github.com/tree-sitter-grammars/tree-sitter-kotlin v1.1.0
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-java v0.23.5
)

exclude (
	github.com/ebitenui/ebitenui v0.6.2
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394
	golang.org/x/image v0.25.0
	golang.org/x/sync v0.12.0
	golang.org/x/sys v0.31.0
	golang.org/x/text v0.23.0
)

replace doc_generator => ../../../
