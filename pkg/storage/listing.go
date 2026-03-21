package storage

import (
	"os"
	"regexp"
	"slices"
	"strconv"
)

// ListNumberedFiles reads dir and returns sorted numbers extracted from filenames
// matching pattern. The pattern must have a capturing group for the number.
// Returns an empty slice (not nil) if the directory does not exist.
func ListNumberedFiles(dir string, pattern *regexp.Regexp) ([]int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []int{}, nil
		}

		return nil, err
	}

	var numbers []int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := pattern.FindStringSubmatch(entry.Name())
		if len(matches) > 1 {
			num, _ := strconv.Atoi(matches[1])
			numbers = append(numbers, num)
		}
	}

	slices.Sort(numbers)

	return numbers, nil
}
