package main

import (
	"doc_generator/pkg/parsers"
	"doc_generator/pkg/store"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

func main() {
	inputPath := os.Args[1]

	source := store.Source{}

	filepath.WalkDir(inputPath, func(fPath string, d fs.DirEntry, err error) error {

		fileType := path.Ext(fPath)

		if fileType == ".cpp" || fileType == ".hpp" || fileType == ".h" || fileType == ".c" {
			file, err := os.ReadFile(fPath)
			if err != nil {
				panic(err)
			}

			parser := &parsers.CPlusPlus{FileName: fPath, File: file}
			parser.Parse(&source)
		}

		return nil
	})

	fmt.Println(source)
}
