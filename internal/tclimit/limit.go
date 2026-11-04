// Package tclimit reconciles shared per-port, per-direction Linux HTB limits
// across protocols and selected interfaces without restarting Xray.
package tclimit

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// MaxMbps is the public API limit, shared with model validation and the UI.
const MaxMbps = 10000

type Limit struct {
	Port int
	Mbps int
}

// Requests coalesces edits. The server also reconciles periodically and at boot.
var Requests = make(chan struct{}, 1)

func Notify() {
	select {
	case Requests <- struct{}{}:
	default:
	}
}

func Validate(limit Limit) error {
	if limit.Mbps < 0 || limit.Mbps > MaxMbps {
		return fmt.Errorf("speedLimit must be an integer between 0 and %d Mbps", MaxMbps)
	}
	if limit.Mbps > 0 && (limit.Port < 1 || limit.Port > 65535) {
		return fmt.Errorf("tc speedLimit requires a TCP/UDP port between 1 and 65535")
	}
	return nil
}

func runTC(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Arguments never pass through a shell. Do not include configuration or
	// credentials in errors; tc only receives interface names and numeric traffic-control parameters here.
	out, err := exec.CommandContext(ctx, "tc", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tc %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
