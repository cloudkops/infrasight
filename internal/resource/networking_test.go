package resource

import "testing"

func TestNetworking_Flatten_KnownTypesGetExplicitBooleans(t *testing.T) {
	n := Networking{Exposures: []Exposure{{Type: "hostNetwork"}}}
	attrs := n.Flatten("networking")

	if attrs["networking.hostNetwork"] != true {
		t.Errorf("expected hostNetwork true, got %v", attrs["networking.hostNetwork"])
	}
	// Known types not present must still resolve, explicitly false — so
	// "equals: false" works even when a resource isn't exposed that way.
	if attrs["networking.publicIP"] != false {
		t.Errorf("expected publicIP false, got %v", attrs["networking.publicIP"])
	}
	if attrs["networking.nodePort"] != false {
		t.Errorf("expected nodePort false, got %v", attrs["networking.nodePort"])
	}
	if attrs["networking.loadBalancer"] != false {
		t.Errorf("expected loadBalancer false, got %v", attrs["networking.loadBalancer"])
	}
}

func TestNetworking_Flatten_UnknownTypeIsQueryableAsTrue(t *testing.T) {
	n := Networking{Exposures: []Exposure{{Type: "port"}}}
	attrs := n.Flatten("networking")

	if attrs["networking.port"] != true {
		t.Errorf("expected an unlisted exposure type to still be queryable, got %v", attrs["networking.port"])
	}
}
