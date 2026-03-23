package git

import "testing"

func TestParseUnifiedDiffHunks(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -10,0 +11,3 @@ func foo() {
+	line1
+	line2
+	line3
@@ -50,2 +54,4 @@ func bar() {
+	new1
+	new2
+	new3
+	new4
diff --git a/util.go b/util.go
index 1234567..89abcde 100644
--- a/util.go
+++ b/util.go
@@ -1,5 +1,7 @@ package util
+added line
+another line
`

	result := parseUnifiedDiffHunks(diff)

	// main.go should have two hunks
	mainHunks := result["main.go"]
	if len(mainHunks) != 2 {
		t.Fatalf("main.go: expected 2 hunks, got %d", len(mainHunks))
	}
	if mainHunks[0] != [2]int{11, 13} {
		t.Errorf("main.go hunk[0] = %v, want [11, 13]", mainHunks[0])
	}
	if mainHunks[1] != [2]int{54, 57} {
		t.Errorf("main.go hunk[1] = %v, want [54, 57]", mainHunks[1])
	}

	// util.go should have one hunk
	utilHunks := result["util.go"]
	if len(utilHunks) != 1 {
		t.Fatalf("util.go: expected 1 hunk, got %d", len(utilHunks))
	}
	if utilHunks[0] != [2]int{1, 7} {
		t.Errorf("util.go hunk[0] = %v, want [1, 7]", utilHunks[0])
	}
}

func TestParseUnifiedDiffHunksSingleLine(t *testing.T) {
	diff := `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -5,0 +6 @@ func x() {
+	newLine
`

	result := parseUnifiedDiffHunks(diff)

	hunks := result["file.go"]
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	// When count is omitted it defaults to 1
	if hunks[0] != [2]int{6, 6} {
		t.Errorf("hunk = %v, want [6, 6]", hunks[0])
	}
}

func TestParseUnifiedDiffHunksDeletionOnly(t *testing.T) {
	diff := `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -10,3 +10,0 @@ func x() {
-	removed1
-	removed2
-	removed3
`

	result := parseUnifiedDiffHunks(diff)

	hunks := result["file.go"]
	if len(hunks) != 0 {
		t.Errorf("expected 0 hunks for pure deletion, got %d", len(hunks))
	}
}

func TestParseUnifiedDiffHunksEmpty(t *testing.T) {
	result := parseUnifiedDiffHunks("")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}
