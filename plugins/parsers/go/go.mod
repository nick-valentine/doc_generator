module doc_generator/plugins/parsers/go

go 1.22.0

require (
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-go v0.23.4
	doc_generator v0.0.0
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
