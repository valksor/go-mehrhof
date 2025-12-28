package wrike

import (
	"github.com/valksor/go-mehrhof/internal/provider"
)

// Info returns the provider information
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        "wrike",
		Description: "Wrike task source",
		Schemes:     []string{"wrike", "wk"},
		Priority:    20, // Same as GitHub
		Capabilities: provider.CapabilitySet{
			provider.CapRead:               true,
			provider.CapFetchComments:      true,
			provider.CapComment:            true,
			provider.CapDownloadAttachment: true,
			provider.CapSnapshot:           true,
		},
	}
}

// Register adds the Wrike provider to the registry
func Register(r *provider.Registry) {
	_ = r.Register(Info(), New)
}
