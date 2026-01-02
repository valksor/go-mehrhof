package azuredevops

import "github.com/valksor/go-mehrhof/internal/provider"

// Register adds the Azure DevOps provider to the registry.
func Register(r *provider.Registry) {
	_ = r.Register(Info(), New)
}
