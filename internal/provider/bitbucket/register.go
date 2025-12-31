package bitbucket

import "github.com/valksor/go-mehrhof/internal/provider"

// Register adds the Bitbucket provider to the registry
func Register(r *provider.Registry) {
	_ = r.Register(Info(), New)
}
