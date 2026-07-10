package gpu

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseCUDADevices validates and normalizes a comma-separated list of CUDA
// device indices. An empty list means the parent environment should be used.
func ParseCUDADevices(s string) (count int, normalized string, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", nil
	}

	tokens := strings.Split(s, ",")
	normalizedIDs := make([]string, 0, len(tokens))
	seen := make(map[uint64]struct{}, len(tokens))

	for i, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			return 0, "", fmt.Errorf("empty CUDA device ID at position %d", i+1)
		}
		if strings.HasPrefix(token, "-") {
			return 0, "", fmt.Errorf("CUDA device ID %q must not be negative", token)
		}
		for _, char := range token {
			if char < '0' || char > '9' {
				return 0, "", fmt.Errorf("CUDA device ID %q is not a decimal integer", token)
			}
		}

		id, parseErr := strconv.ParseUint(token, 10, 64)
		if parseErr != nil {
			return 0, "", fmt.Errorf("invalid CUDA device ID %q: %w", token, parseErr)
		}
		if _, exists := seen[id]; exists {
			return 0, "", fmt.Errorf("duplicate CUDA device ID %d", id)
		}
		seen[id] = struct{}{}
		normalizedIDs = append(normalizedIDs, strconv.FormatUint(id, 10))
	}

	return len(normalizedIDs), strings.Join(normalizedIDs, ","), nil
}
