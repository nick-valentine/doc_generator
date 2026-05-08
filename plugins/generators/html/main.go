package main

import (
	"doc_generator/pkg/generators"
	"doc_generator/pkg/store"
)

// Generator is the exported generator implementation
var Generator store.Generator = &generators.HTMLGenerator{}

// Format is the output format of this generator
var Format = "html"
