package store

type Source struct {
	Files []File
}

func (s *Source) GetFile(name string) *File {
	for i := range s.Files {
		if s.Files[i].Name == name {
			return &s.Files[i]
		}
	}
	return nil
}

type File struct {
	Name        string
	FileImports []File
}

func (f *File) AddFileImport(impt File) {
	f.FileImports = append(f.FileImports, impt)
}
