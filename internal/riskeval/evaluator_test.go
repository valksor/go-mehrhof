package riskeval

import (
	"math"
	"testing"
)

func TestEvaluate_SmallREADMEChange(t *testing.T) {
	score := Evaluate(Input{
		DiffLinesAdded:    3,
		DiffLinesRemoved:  1,
		FilesChanged:      []string{"README.md"},
		TotalFilesChanged: 1,
		TestFilesChanged:  0,
	})

	if score.Level != LevelLow {
		t.Errorf("expected low risk for small README change, got %s (score=%.3f)", score.Level, score.Score)
	}
	if score.Score >= DefaultLowThreshold {
		t.Errorf("expected score < %.1f, got %.3f", DefaultLowThreshold, score.Score)
	}
}

func TestEvaluate_LargeAuthRefactor(t *testing.T) {
	score := Evaluate(Input{
		DiffLinesAdded:   300,
		DiffLinesRemoved: 200,
		FilesChanged: []string{
			"pkg/auth/handler.go",
			"pkg/auth/middleware.go",
			"pkg/auth/tokens.go",
			"pkg/auth/session.go",
			"pkg/auth/oauth.go",
			"pkg/auth/config.go",
			"pkg/api/routes.go",
			"pkg/api/middleware.go",
			"pkg/db/migrations/0042_auth.sql",
			"pkg/db/models/user.go",
		},
		SensitivePaths:    []string{"pkg/auth/*"},
		SecurityFindings:  1,
		TotalFilesChanged: 10,
		TestFilesChanged:  0,
	})

	if score.Level != LevelHigh {
		t.Errorf("expected high risk for large auth refactor, got %s (score=%.3f)", score.Level, score.Score)
	}
	if score.Score < DefaultHighThreshold {
		t.Errorf("expected score >= %.1f, got %.3f", DefaultHighThreshold, score.Score)
	}
}

func TestEvaluate_SecurityFindingsBoost(t *testing.T) {
	baseInput := Input{
		DiffLinesAdded:    50,
		DiffLinesRemoved:  10,
		FilesChanged:      []string{"pkg/handler.go"},
		TotalFilesChanged: 1,
		TestFilesChanged:  0,
	}

	withoutFindings := Evaluate(baseInput)

	withFindings := baseInput
	withFindings.SecurityFindings = 2
	scoreWithFindings := Evaluate(withFindings)

	if scoreWithFindings.Score <= withoutFindings.Score {
		t.Errorf("security findings should increase score: without=%.3f, with=%.3f",
			withoutFindings.Score, scoreWithFindings.Score)
	}

	if scoreWithFindings.Factors[FactorSecurityFindings] != 1.0 {
		t.Errorf("expected security_findings factor = 1.0, got %.3f",
			scoreWithFindings.Factors[FactorSecurityFindings])
	}
}

func TestEvaluate_AutoApproveThreshold(t *testing.T) {
	// A tiny change with no sensitive files should score well below auto-approve threshold.
	score := Evaluate(Input{
		DiffLinesAdded:    5,
		DiffLinesRemoved:  2,
		FilesChanged:      []string{"docs/guide.md"},
		TotalFilesChanged: 1,
		TestFilesChanged:  0,
	})

	if score.Score >= DefaultLowThreshold {
		t.Errorf("expected auto-approvable score < %.1f, got %.3f", DefaultLowThreshold, score.Score)
	}
}

func TestEvaluate_HighTestRatio(t *testing.T) {
	// Half test files should reduce risk.
	withTests := Evaluate(Input{
		DiffLinesAdded:    100,
		DiffLinesRemoved:  50,
		FilesChanged:      []string{"pkg/handler.go", "pkg/handler_test.go"},
		TotalFilesChanged: 2,
		TestFilesChanged:  1,
	})

	withoutTests := Evaluate(Input{
		DiffLinesAdded:    100,
		DiffLinesRemoved:  50,
		FilesChanged:      []string{"pkg/handler.go", "pkg/util.go"},
		TotalFilesChanged: 2,
		TestFilesChanged:  0,
	})

	if withTests.Score >= withoutTests.Score {
		t.Errorf("higher test ratio should lower risk: with_tests=%.3f, without=%.3f",
			withTests.Score, withoutTests.Score)
	}
}

func TestDefaultWeights(t *testing.T) {
	weights := DefaultWeights()

	var sum float64
	for _, w := range weights {
		sum += w
	}

	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("weights should sum to 1.0, got %.3f", sum)
	}

	expectedFactors := []string{
		FactorDiffSize,
		FactorSensitivePaths,
		FactorSecurityFindings,
		FactorFileCount,
		FactorTestRatio,
	}
	for _, f := range expectedFactors {
		if _, ok := weights[f]; !ok {
			t.Errorf("missing weight for factor %q", f)
		}
	}
}

func TestClassifyLevel(t *testing.T) {
	tests := []struct {
		name  string
		score float64
		want  string
	}{
		{"zero", 0.0, LevelLow},
		{"just_below_low", 0.29, LevelLow},
		{"at_low_boundary", 0.3, LevelMedium},
		{"mid_medium", 0.5, LevelMedium},
		{"just_below_high", 0.69, LevelMedium},
		{"at_high_boundary", 0.7, LevelHigh},
		{"max", 1.0, LevelHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyLevel(tt.score)
			if got != tt.want {
				t.Errorf("classifyLevel(%.2f) = %q, want %q", tt.score, got, tt.want)
			}
		})
	}
}

func TestScoreDiffSize(t *testing.T) {
	tests := []struct {
		name  string
		lines int
		want  float64
	}{
		{"zero", 0, 0.0},
		{"small", 10, 0.0},
		{"at_threshold", 20, 0.0},
		{"mid", 260, 0.5},
		{"large", 500, 1.0},
		{"huge", 1000, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreDiffSize(tt.lines)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("scoreDiffSize(%d) = %.3f, want %.3f", tt.lines, got, tt.want)
			}
		})
	}
}

func TestScoreFileCount(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  float64
	}{
		{"zero", 0, 0.0},
		{"one", 1, 0.0},
		{"ten", 10, float64(9) / float64(19)},
		{"twenty", 20, 1.0},
		{"many", 50, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreFileCount(tt.count)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("scoreFileCount(%d) = %.3f, want %.3f", tt.count, got, tt.want)
			}
		})
	}
}

func TestScoreTestRatio(t *testing.T) {
	tests := []struct {
		name       string
		testFiles  int
		totalFiles int
		want       float64
	}{
		{"no_files", 0, 0, 0.0},
		{"no_tests", 0, 4, 1.0},
		{"half_tests", 2, 4, 0.0},
		{"quarter_tests", 1, 4, 0.5},
		{"all_tests", 3, 3, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreTestRatio(tt.testFiles, tt.totalFiles)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("scoreTestRatio(%d, %d) = %.3f, want %.3f",
					tt.testFiles, tt.totalFiles, got, tt.want)
			}
		})
	}
}

func TestScoreSensitivePaths(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		patterns []string
		want     float64
	}{
		{"no_files", nil, []string{"pkg/auth/*"}, 0.0},
		{"no_patterns", []string{"pkg/auth/handler.go"}, nil, 0.0},
		{"all_match", []string{"pkg/auth/handler.go", "pkg/auth/middleware.go"}, []string{"pkg/auth/*"}, 1.0},
		{"half_match", []string{"pkg/auth/handler.go", "pkg/util/helpers.go"}, []string{"pkg/auth/*"}, 0.5},
		{"none_match", []string{"pkg/util/helpers.go"}, []string{"pkg/auth/*"}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreSensitivePaths(tt.files, tt.patterns)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("scoreSensitivePaths() = %.3f, want %.3f", got, tt.want)
			}
		})
	}
}

func TestEvaluateWithWeights_CustomWeights(t *testing.T) {
	// Put all weight on security findings
	weights := map[string]float64{
		FactorSecurityFindings: 1.0,
	}

	score := EvaluateWithWeights(Input{
		DiffLinesAdded:    500,
		DiffLinesRemoved:  500,
		TotalFilesChanged: 30,
		SecurityFindings:  0,
	}, weights)

	if score.Score != 0.0 {
		t.Errorf("with all weight on security and no findings, expected 0.0, got %.3f", score.Score)
	}

	score = EvaluateWithWeights(Input{
		SecurityFindings: 1,
	}, weights)

	if score.Score != 1.0 {
		t.Errorf("with all weight on security and findings present, expected 1.0, got %.3f", score.Score)
	}
}
