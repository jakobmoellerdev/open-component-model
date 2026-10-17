package plugins

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"ocm.software/open-component-model/bindings/go/plugin/internal/osutil"
)

// interruptTimeout bounds the request asking a plugin to shut down. The plugin answers before shutting down.
const interruptTimeout = 5 * time.Second

// Interrupt asks the plugin to shut down gracefully through its shutdown endpoint, which the
// plugin SDK serves, because Windows cannot deliver os.Interrupt to another process.
// If the plugin cannot be reached, it is killed. It returns os.ErrProcessDone if the plugin already exited.
func (p Process) Interrupt(ctx context.Context) error {
	// Like sending a signal, delivering the request must not fail because the caller's context is already done.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), interruptTimeout)
	defer cancel()

	err := Call(ctx, p.Client, p.Type, p.Location, "shutdown", http.MethodGet)
	if err == nil {
		return nil
	}
	if !osutil.ProcessAlive(p.Cmd.Process.Pid) {
		return os.ErrProcessDone
	}

	return errors.Join(fmt.Errorf("failed to request plugin shutdown: %w", err), p.Cmd.Process.Kill())
}
