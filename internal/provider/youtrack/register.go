package youtrack

import "github.com/valksor/go-mehrhof/internal/provider"

const (
	// ProviderName is the registered name for this provider
	ProviderName = "youtrack"
)

// Info returns the provider information
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "YouTrack issue tracker",
		Schemes:     []string{"youtrack", "yt"},
		Priority:    20, // Same as GitHub/Wrike
		Capabilities: provider.CapabilitySet{
			provider.CapRead:               true,
			provider.CapList:               true,
			provider.CapFetchComments:      true,
			provider.CapComment:            true,
			provider.CapUpdateStatus:       true,
			provider.CapManageLabels:       true,
			provider.CapCreateWorkUnit:     true,
			provider.CapDownloadAttachment: true,
			provider.CapSnapshot:           true,
		},
	}
}

// Register adds the YouTrack provider to the registry
func Register(r *provider.Registry) {
	_ = r.Register(Info(), New)
}
