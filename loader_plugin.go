//go:build !windows && !js && !wasip1

package tengo

import (
	"fmt"
	"path/filepath"
	"plugin"
	"sync"
)

// PluginLoader loads Go plugin modules (.so files) from a list of directories.
// It implements Loader and can be registered on a ModuleMap via AddLoader.
//
// The plugin must export a package-level variable named TengoModule:
//
//	package main
//
//	import "github.com/tengolang/tengo/v3"
//
//	var TengoModule = map[string]tengo.Object{
//	    "hello": &tengo.UserFunction{...},
//	}
//
// Build the plugin with:
//
//	go build -buildmode=plugin -o mymod.so ./mymod/
//
// The plugin and the host binary must be built with the same Go toolchain and
// the same version of github.com/tengolang/tengo/v3.
//
// Note: Go plugins are not supported on Windows.
type PluginLoader struct {
	dirs  []string
	cache sync.Map // name -> Importable
}

// NewPluginLoader creates a PluginLoader that searches the given directories.
func NewPluginLoader(dirs ...string) *PluginLoader {
	return &PluginLoader{dirs: dirs}
}

// Load searches for <name>.so in each configured directory and loads it as a
// Go plugin. Returns (nil, nil) if no matching file is found.
func (l *PluginLoader) Load(name string) (Importable, error) {
	if v, ok := l.cache.Load(name); ok {
		return v.(Importable), nil
	}
	for _, dir := range l.dirs {
		path := filepath.Join(dir, name+".so")
		p, err := plugin.Open(path)
		if err != nil {
			continue
		}
		sym, err := p.Lookup("TengoModule")
		if err != nil {
			return nil, fmt.Errorf("plugin %s: missing TengoModule symbol", path)
		}
		mod, ok := sym.(*map[string]Object)
		if !ok {
			return nil, fmt.Errorf(
				"plugin %s: TengoModule has wrong type %T, want *map[string]tengo.Object",
				path, sym)
		}
		importable := &BuiltinModule{Attrs: *mod}
		l.cache.Store(name, importable)
		return importable, nil
	}
	return nil, nil
}
