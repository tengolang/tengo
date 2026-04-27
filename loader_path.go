package tengo

import (
	"os"
	"path/filepath"
)

// PathLoader searches a list of directories for Tengo source modules
// (<name>.tengo). It implements Loader and can be registered on a ModuleMap
// via AddLoader.
//
// Search order matches registration order of directories. The first file
// found wins; (nil, nil) is returned when no directory contains the module.
type PathLoader struct {
	dirs []string
}

// NewPathLoader creates a PathLoader that searches the given directories.
func NewPathLoader(dirs ...string) *PathLoader {
	return &PathLoader{dirs: dirs}
}

// Load searches for <name>.tengo in each configured directory.
func (l *PathLoader) Load(name string) (Importable, error) {
	for _, dir := range l.dirs {
		path := filepath.Join(dir, name+".tengo")
		src, err := os.ReadFile(path)
		if err == nil {
			return &SourceModule{Src: src}, nil
		}
	}
	return nil, nil
}
