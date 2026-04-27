package tengo

// Importable interface represents importable module instance.
type Importable interface {
	// Import should return either an Object or module source code ([]byte).
	Import(moduleName string) (interface{}, error)
}

// ModuleGetter enables implementing dynamic module loading.
type ModuleGetter interface {
	Get(name string) Importable
}

// Loader is a fallback module loader. It is called by ModuleMap.Get when a
// module name is not found in the static map. Return (nil, nil) to indicate
// the module was not found by this loader so the next one is tried.
type Loader interface {
	Load(name string) (Importable, error)
}

// errImportable surfaces a loader error through the Importable interface so
// it propagates as a compile error rather than a silent "not found".
type errImportable struct{ err error }

func (e *errImportable) Import(string) (interface{}, error) { return nil, e.err }

// ModuleMap represents a set of named modules. Use NewModuleMap to create a
// new module map.
type ModuleMap struct {
	m       map[string]Importable
	loaders []Loader
}

// NewModuleMap creates a new module map.
func NewModuleMap() *ModuleMap {
	return &ModuleMap{
		m: make(map[string]Importable),
	}
}

// Add adds an import module.
func (m *ModuleMap) Add(name string, module Importable) {
	m.m[name] = module
}

// AddBuiltinModule adds a builtin module.
func (m *ModuleMap) AddBuiltinModule(name string, attrs map[string]Object) {
	m.m[name] = &BuiltinModule{Attrs: attrs}
}

// AddSourceModule adds a source module.
func (m *ModuleMap) AddSourceModule(name string, src []byte) {
	m.m[name] = &SourceModule{Src: src}
}

// AddLoader appends a fallback Loader. Loaders are tried in registration
// order after the static map is checked. The first loader that returns a
// non-nil Importable wins; a non-nil error is surfaced as a compile error.
func (m *ModuleMap) AddLoader(l Loader) {
	m.loaders = append(m.loaders, l)
}

// Remove removes a named module.
func (m *ModuleMap) Remove(name string) {
	delete(m.m, name)
}

// Get returns an import module identified by name. If the name is not in the
// static map, registered Loaders are tried in order. Returns nil if nothing
// claims the name.
func (m *ModuleMap) Get(name string) Importable {
	if mod := m.m[name]; mod != nil {
		return mod
	}
	for _, l := range m.loaders {
		mod, err := l.Load(name)
		if err != nil {
			return &errImportable{err}
		}
		if mod != nil {
			return mod
		}
	}
	return nil
}

// GetBuiltinModule returns a builtin module identified by name. It returns
// if the name is not found or the module is not a builtin module.
func (m *ModuleMap) GetBuiltinModule(name string) *BuiltinModule {
	mod, _ := m.m[name].(*BuiltinModule)
	return mod
}

// GetSourceModule returns a source module identified by name. It returns if
// the name is not found or the module is not a source module.
func (m *ModuleMap) GetSourceModule(name string) *SourceModule {
	mod, _ := m.m[name].(*SourceModule)
	return mod
}

// Copy creates a copy of the module map. Loaders are shared (not deep-copied)
// since they are expected to be stateless or internally synchronized.
func (m *ModuleMap) Copy() *ModuleMap {
	c := &ModuleMap{
		m:       make(map[string]Importable, len(m.m)),
		loaders: m.loaders,
	}
	for name, mod := range m.m {
		c.m[name] = mod
	}
	return c
}

// Len returns the number of named modules.
func (m *ModuleMap) Len() int {
	return len(m.m)
}

// AddMap adds named modules from another module map.
func (m *ModuleMap) AddMap(o *ModuleMap) {
	for name, mod := range o.m {
		m.m[name] = mod
	}
}

// SourceModule is an importable module that's written in Tengo.
type SourceModule struct {
	Src []byte
}

// Import returns a module source code.
func (m *SourceModule) Import(_ string) (interface{}, error) {
	return m.Src, nil
}
