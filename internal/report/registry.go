package report

import "fmt"

var registry = map[string]Renderer{}

// Register adds a renderer under its own Name(). Called from each format package's
// own init() (table/json/sarif/markdown) — internal/app resolves a renderer by
// --format name, never a hardcoded switch.
func Register(r Renderer) {
	registry[r.Name()] = r
}

// Get resolves a renderer by format name.
func Get(name string) (Renderer, error) {
	r, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("report: unknown format %q", name)
	}
	return r, nil
}

// List returns the names of every registered renderer.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
