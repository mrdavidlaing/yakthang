package runtime

import (
	"context"
	"fmt"
	"time"
)

// SpawnSandboxWorker spawns a worker in the sandbox runtime.
// This is a stub — the actual implementation will be added in later yaks.
func SpawnSandboxWorker(ctx context.Context, opts ...SpawnOption) error {
	return fmt.Errorf("sandbox runtime not yet implemented")
}

// StopSandboxWorker stops a sandbox worker.
// This is a stub — the actual implementation will be added in later yaks.
func StopSandboxWorker(name string, timeout time.Duration) error {
	return fmt.Errorf("sandbox runtime not yet implemented")
}
