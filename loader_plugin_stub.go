//go:build windows || js || wasip1

package tengo

// PluginLoader is a no-op stub on platforms that do not support Go plugins
// (Windows, js/wasm, wasip1). NewPluginLoader returns a loader that always
// reports no module found.
type PluginLoader struct{}

// NewPluginLoader returns a stub PluginLoader on unsupported platforms.
func NewPluginLoader(_ ...string) *PluginLoader { return &PluginLoader{} }

// Load always returns (nil, nil) on unsupported platforms.
func (l *PluginLoader) Load(_ string) (Importable, error) { return nil, nil }
