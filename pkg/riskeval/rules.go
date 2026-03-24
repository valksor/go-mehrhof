package riskeval

// Factor name constants used as keys in weight maps and RiskScore.Factors.
const (
	FactorDiffSize         = "diff_size"
	FactorSensitivePaths   = "sensitive_paths"
	FactorSecurityFindings = "security_findings"
	FactorFileCount        = "file_count"
	FactorTestRatio        = "test_ratio"
)

// DefaultWeights returns the default risk factor weights.
// Weights sum to 1.0.
func DefaultWeights() map[string]float64 {
	return map[string]float64{
		FactorDiffSize:         0.30,
		FactorSensitivePaths:   0.25,
		FactorSecurityFindings: 0.25,
		FactorFileCount:        0.10,
		FactorTestRatio:        0.10,
	}
}
