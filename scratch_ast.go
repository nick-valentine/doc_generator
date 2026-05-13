package main

import (
	"doc_generator/pkg/store"
	"fmt"
)

func getSymbolURL(fullName string, source *store.Source) string {
	sym := source.FindSymbolByFullName(fullName)
	if sym == nil {
		return ""
	}
	pkg := sym.Package
	if pkg == "" {
		pkg = "main"
	}
	switch sym.Kind {
	case store.SymStruct:
		return fmt.Sprintf("../pages/pkg_%s.html#struct_%s", pkg, sym.Name)
	case store.SymInterface:
		return fmt.Sprintf("../pages/pkg_%s.html#interface_%s", pkg, sym.Name)
	case store.SymMethod:
		return fmt.Sprintf("../pages/pkg_%s.html#func_%s_%s", pkg, sym.Parent, sym.Name)
	case store.SymFunction:
		return fmt.Sprintf("../pages/pkg_%s.html#func_%s", pkg, sym.Name)
	}
	return ""
}

func getDotURLAttr(fullName string, source *store.Source) string {
	url := getSymbolURL(fullName, source)
	if url == "" {
		return ""
	}
	return fmt.Sprintf(", URL=\"%s\", target=\"_top\"", url)
}

func main() {
	source := &store.Source{
		Symbols: []store.Symbol{
			{
				Name:    "ParseCoverage",
				Kind:    store.SymFunction,
				Package: "store",
			},
		},
	}

	source.BuildIndexes()

	fmt.Println("Testing store.ParseCoverage lookup:")
	urlAttr := getDotURLAttr("store.ParseCoverage", source)
	fmt.Printf("Result: '%s'\n", urlAttr)
}
