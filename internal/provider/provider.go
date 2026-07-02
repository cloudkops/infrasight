package provider

import (
	"context"

	"github.com/cloudkops/infrasight/internal/resource"
)

// Kind distinguishes providers that connect to a live target from providers that
// scan static configuration with no live target to reach.
type Kind string

const (
	KindLive   Kind = "live"   // kubernetes, docker, host
	KindStatic Kind = "static" // terraform, ansible
)

// Options carries every per-provider connection/discovery setting. Not every field
// applies to every provider; a provider reads only the fields it understands.
type Options struct {
	// Common
	Namespace     string
	AllNamespaces bool
	LabelSelector string
	FieldSelector string
	Verbose       bool

	// Live-provider specifics
	Kubeconfig string
	Context    string
	DockerHost string

	// Static-provider specifics
	Path string // playbook dir, .tf dir, or plan JSON file
}

// Provider is the single contract every infrastructure source implements. The
// orchestrator depends only on this interface — never on a concrete providers/*
// package.
type Provider interface {
	Name() string
	Kind() Kind
	Discover(ctx context.Context, opts Options) ([]resource.Resource, error)
}
