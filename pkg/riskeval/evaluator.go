// Package riskeval provides task-level risk scoring for auto-approval decisions.
//
// Risk scores range from 0.0 (safe) to 1.0 (dangerous) and are computed from
// multiple weighted factors: diff size, sensitive path overlap, security findings,
// file count, and test coverage ratio. The resulting level ("low", "medium", "high")
// determines whether a transition can be auto-approved or requires manual review.
package riskeval

import (
	"math"
	"path/filepath"
)

// RiskScore represents the evaluated risk of a task change.
type RiskScore struct {
	Score   float64            `json:"score"`   // 0.0 (safe) to 1.0 (dangerous)
	Factors map[string]float64 `json:"factors"` // Per-factor scores
	Level   string             `json:"level"`   // "low", "medium", "high"
}

// Input provides the data needed for risk evaluation.
type Input struct {
	DiffLinesAdded    int
	DiffLinesRemoved  int
	FilesChanged      []string
	SensitivePaths    []string // Glob patterns from policy settings
	SecurityFindings  int
	TestFilesChanged  int
	TotalFilesChanged int
}

// LevelLow is the risk level for scores below the low threshold.
const LevelLow = "low"

// LevelMedium is the risk level for scores between low and high thresholds.
const LevelMedium = "medium"

// LevelHigh is the risk level for scores at or above the high threshold.
const LevelHigh = "high"

// DefaultLowThreshold is the default boundary below which risk is considered low.
const DefaultLowThreshold = 0.3

// DefaultHighThreshold is the default boundary at or above which risk is considered high.
const DefaultHighThreshold = 0.7

// Evaluate computes a risk score from the given input using default weights.
func Evaluate(input Input) RiskScore {
	return EvaluateWithWeights(input, DefaultWeights())
}

// EvaluateWithWeights computes a risk score using the provided factor weights.
func EvaluateWithWeights(input Input, weights map[string]float64) RiskScore {
	factors := make(map[string]float64, 5)

	factors[FactorDiffSize] = scoreDiffSize(input.DiffLinesAdded + input.DiffLinesRemoved)
	factors[FactorSensitivePaths] = scoreSensitivePaths(input.FilesChanged, input.SensitivePaths)
	factors[FactorSecurityFindings] = scoreSecurityFindings(input.SecurityFindings)
	factors[FactorFileCount] = scoreFileCount(input.TotalFilesChanged)
	factors[FactorTestRatio] = scoreTestRatio(input.TestFilesChanged, input.TotalFilesChanged)

	var score float64
	for factor, value := range factors {
		w, ok := weights[factor]
		if !ok {
			continue
		}
		score += value * w
	}

	// Clamp to [0, 1]
	score = math.Max(0, math.Min(1, score))

	return RiskScore{
		Score:   score,
		Factors: factors,
		Level:   classifyLevel(score),
	}
}

func classifyLevel(score float64) string {
	switch {
	case score < DefaultLowThreshold:
		return LevelLow
	case score < DefaultHighThreshold:
		return LevelMedium
	default:
		return LevelHigh
	}
}

// scoreDiffSize: >500 lines = 1.0, <20 lines = 0.0, linear between.
func scoreDiffSize(totalLines int) float64 {
	if totalLines <= 20 {
		return 0.0
	}
	if totalLines >= 500 {
		return 1.0
	}

	return float64(totalLines-20) / float64(500-20)
}

// scoreSensitivePaths: fraction of changed files matching sensitive path patterns.
func scoreSensitivePaths(files, patterns []string) float64 {
	if len(files) == 0 || len(patterns) == 0 {
		return 0.0
	}

	matched := 0
	for _, f := range files {
		for _, pattern := range patterns {
			if ok, _ := filepath.Match(pattern, f); ok {
				matched++

				break
			}
			// Also match against base name for patterns like "*.env"
			if ok, _ := filepath.Match(pattern, filepath.Base(f)); ok {
				matched++

				break
			}
		}
	}

	return float64(matched) / float64(len(files))
}

// scoreSecurityFindings: any security finding = 1.0, none = 0.0.
func scoreSecurityFindings(count int) float64 {
	if count > 0 {
		return 1.0
	}

	return 0.0
}

// scoreFileCount: >20 files = 1.0, 1 file = 0.0, linear.
func scoreFileCount(count int) float64 {
	if count <= 1 {
		return 0.0
	}
	if count >= 20 {
		return 1.0
	}

	return float64(count-1) / float64(20-1)
}

// scoreTestRatio: 0 test files = 1.0, >50% test files = 0.0, linear between.
func scoreTestRatio(testFiles, totalFiles int) float64 {
	if totalFiles == 0 {
		return 0.0
	}
	ratio := float64(testFiles) / float64(totalFiles)
	if ratio >= 0.5 {
		return 0.0
	}

	return 1.0 - (ratio / 0.5)
}
