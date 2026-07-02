package provider

import "context"

// Manager is the single resolution contract internal/app and internal/scan depend
// on. Providers are first-class: resolved by name through Manager, never imported
// directly outside their own providers/* package.
type Manager struct{}

// NewManager constructs a Manager over the global provider registry.
func NewManager() *Manager {
	return &Manager{}
}

// Get resolves a provider by name.
func (m *Manager) Get(name string) (Provider, error) {
	return Find(name)
}

// List returns the names of every registered provider.
func (m *Manager) List() []string {
	return List()
}

// Initialize is a lifecycle hook for providers that need to establish state before
// Discover is called. No provider needs this yet; kept as a no-op so the lifecycle
// contract (Load -> Initialize -> Discover -> Shutdown) is in place before it's
// needed.
func (m *Manager) Initialize(ctx context.Context, p Provider) error {
	return nil
}

// Shutdown is the matching lifecycle hook for releasing provider state.
func (m *Manager) Shutdown(ctx context.Context, p Provider) error {
	return nil
}
