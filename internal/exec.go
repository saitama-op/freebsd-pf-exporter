// internal/exec.go
package internal

import (
	"context"
	"os/exec"
)

// RunCommand executes a shell command with a timeout context and returns its standard output.
func RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}
