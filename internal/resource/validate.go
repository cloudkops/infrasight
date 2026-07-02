package resource

import "fmt"

// Validate checks the structural invariants every provider's normalize step must
// satisfy before a Resource reaches the Rule Engine.
func Validate(r Resource) error {
	if r.ID == "" {
		return fmt.Errorf("resource: missing ID")
	}
	if r.Name == "" {
		return fmt.Errorf("resource %q: missing Name", r.ID)
	}
	if r.Kind == "" {
		return fmt.Errorf("resource %q: missing Kind", r.ID)
	}
	if r.Provider == "" {
		return fmt.Errorf("resource %q: missing Provider", r.ID)
	}
	return nil
}
