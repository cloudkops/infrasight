package resource

import "testing"

func TestSecurityContext_Flatten(t *testing.T) {
	trueVal := true
	falseVal := false

	sec := SecurityContext{
		RunAsRoot:                true,
		Privileged:               true,
		AllowPrivilegeEscalation: &trueVal,
		ReadOnlyRootFilesystem:   &falseVal,
		CapabilitiesAdd:          []string{"NET_ADMIN"},
		Public:                   true,
	}

	attrs := sec.Flatten("container.security")

	want := map[string]any{
		"container.security.run_as_root":                true,
		"container.security.privileged":                 true,
		"container.security.public":                     true,
		"container.security.allow_privilege_escalation": true,
		"container.security.read_only_root_filesystem":  false,
		"container.security.capabilities_add":           []string{"NET_ADMIN"},
	}

	for k, v := range want {
		got, ok := attrs[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if s, isSlice := v.([]string); isSlice {
			gotSlice, _ := got.([]string)
			if len(gotSlice) != len(s) || gotSlice[0] != s[0] {
				t.Errorf("%s = %v, want %v", k, got, v)
			}
			continue
		}
		if got != v {
			t.Errorf("%s = %v, want %v", k, got, v)
		}
	}

	if _, ok := attrs["container.security.encrypted"]; ok {
		t.Error("expected encrypted to be absent when unset (pointer field, not zero-valued)")
	}
	if _, ok := attrs["container.security.capabilities_drop"]; ok {
		t.Error("expected capabilities_drop to be absent when empty")
	}
}
