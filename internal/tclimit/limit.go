// Package tclimit reconciles port-wide Linux traffic-control policers.
// Each port has one shared token bucket per direction, across IP families,
// TCP/UDP and selected network interfaces. These are kernel tc rules, visible
// with tc; Xray does not need to be patched or restarted to change a rate.
package tclimit

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// MaxMbps keeps the byte rate within the uint32 police ABI used by netlink.
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
	// credentials in errors; tc only receives numeric rates/action IDs here.
	out, err := exec.CommandContext(ctx, "tc", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tc %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
