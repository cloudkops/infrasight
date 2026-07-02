package provider

import (
	"fmt"
	"sort"
)

var registry = map[string]Provider{}

// Register adds a provider under name. Called once at startup from each provider
// package's own init(), never from internal/app or internal/scan directly.
func Register(name string, p Provider) {
	registry[name] = p
}

// Find resolves a provider by name.
func Find(name string) (Provider, error) {
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("provider: unknown provider %q", name)
	}
	return p, nil
}

// List returns the names of every registered provider, sorted.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
