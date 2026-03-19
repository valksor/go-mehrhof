package activitylog

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ChainResult contains the result of a hash chain verification.
type ChainResult struct {
	Valid       bool
	EntriesRead int
	ErrorLine   int    // 1-based line number where chain broke (0 if valid)
	ErrorMsg    string // Human-readable description of the failure
}

// VerifyChain reads all activity log files in the directory and verifies the
// hash chain integrity. Each entry's PrevHash must match the SHA-256 of the
// previous entry's JSON line. Returns a result indicating validity and where
// any break occurred.
func (l *Log) VerifyChain() (*ChainResult, error) {
	files, err := l.logFilesSorted()
	if err != nil {
		return nil, fmt.Errorf("list log files: %w", err)
	}

	result := &ChainResult{Valid: true}
	prevHash := ""
	globalLine := 0

	for _, file := range files {
		path := filepath.Join(l.dir, file)
		lineNum, ph, chainErr := verifyFile(path, prevHash, globalLine)
		globalLine = lineNum
		if chainErr != nil {
			result.Valid = false
			result.ErrorLine = lineNum
			result.ErrorMsg = chainErr.Error()

			break
		}
		prevHash = ph
	}

	result.EntriesRead = globalLine

	return result, nil
}

// verifyFile checks the hash chain within a single JSONL file.
// startLine is the global line offset; prevHash is the expected PrevHash of the first entry.
// Returns (totalLinesProcessed, lastHash, error).
func verifyFile(path, prevHash string, startLine int) (int, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return startLine, prevHash, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lineNum := startLine

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			return lineNum, "", fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}

		// Verify the chain: entry's PrevHash should match our tracked prevHash
		if entry.PrevHash != prevHash {
			return lineNum, "", fmt.Errorf("line %d: chain broken: expected prev_hash %q, got %q", lineNum, prevHash, entry.PrevHash)
		}

		// Compute hash of this entry's JSON line for the next entry
		h := sha256.Sum256(line)
		prevHash = hex.EncodeToString(h[:])
	}

	if err := scanner.Err(); err != nil {
		return lineNum, "", fmt.Errorf("scan %s: %w", path, err)
	}

	return lineNum, prevHash, nil
}

// lastHashFromFile reads the last non-empty line of a JSONL file and returns
// its SHA-256 hash. Returns empty string if the file is empty or doesn't exist.
func lastHashFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	var lastLine []byte
	for scanner.Scan() {
		if line := scanner.Bytes(); len(line) > 0 {
			lastLine = make([]byte, len(line))
			copy(lastLine, line)
		}
	}

	if len(lastLine) == 0 {
		return ""
	}

	h := sha256.Sum256(lastLine)

	return hex.EncodeToString(h[:])
}

// VerifyChainForFile verifies the hash chain in a single activity log file.
func VerifyChainForFile(path string) (*ChainResult, error) {
	lineNum, _, err := verifyFile(path, "", 0)
	result := &ChainResult{
		Valid:       err == nil,
		EntriesRead: lineNum,
	}
	if err != nil {
		result.ErrorLine = lineNum
		result.ErrorMsg = err.Error()
	}

	return result, nil
}

// VerifyChainDir verifies the hash chain across all activity log files in a directory.
func VerifyChainDir(dir string) (*ChainResult, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var logFiles []string
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.HasPrefix(name, filePrefix) && strings.HasSuffix(name, fileExt) {
			logFiles = append(logFiles, name)
		}
	}
	sort.Strings(logFiles)

	result := &ChainResult{Valid: true}
	prevHash := ""
	globalLine := 0

	for _, file := range logFiles {
		path := filepath.Join(dir, file)
		lineNum, ph, chainErr := verifyFile(path, prevHash, globalLine)
		globalLine = lineNum
		if chainErr != nil {
			result.Valid = false
			result.ErrorLine = lineNum
			result.ErrorMsg = chainErr.Error()

			break
		}
		prevHash = ph
	}

	result.EntriesRead = globalLine

	return result, nil
}
