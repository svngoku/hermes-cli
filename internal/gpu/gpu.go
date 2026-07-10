package gpu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/svngoku/hermes-cli/internal/execx"
)

const queryTimeout = 10 * time.Second

var ErrUnavailable = errors.New("nvidia-smi not available")

func CountFromQueryOutput(stdout string) int {
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func Count(ctx context.Context) (int, error) {
	if !execx.CommandExists("nvidia-smi") {
		return 0, ErrUnavailable
	}

	result := execx.RunWithTimeout(
		ctx,
		queryTimeout,
		"nvidia-smi",
		"--query-gpu=index,name,memory.total",
		"--format=csv,noheader",
	)
	if result.ExitCode != 0 {
		return 0, fmt.Errorf("query GPU inventory: nvidia-smi failed: %s", result.Stderr)
	}

	return CountFromQueryOutput(result.Stdout), nil
}
