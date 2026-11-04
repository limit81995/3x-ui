package job

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/tclimit"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type TCSpeedLimitJob struct {
	ctx     context.Context
	mu      sync.Mutex
	manager *tclimit.Manager
}

func NewTCSpeedLimitJob(ctx context.Context) *TCSpeedLimitJob {
	return &TCSpeedLimitJob{ctx: ctx, manager: tclimit.NewManager()}
}

func (j *TCSpeedLimitJob) Run() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.ctx.Err() != nil {
		return
	}
	limits, err := (&service.InboundService{}).DesiredTCSpeedLimits()
	if err != nil {
		logger.Warning("tc speed limit: read desired limits failed:", err)
		return
	}
	// Existing installations without tc and without configured limits keep
	// working. With tc installed, an empty desired set cleans up stale rules.
	if len(limits) == 0 {
		if _, err := exec.LookPath("tc"); err != nil {
			return
		}
	}
	ctx, cancel := context.WithTimeout(j.ctx, 45*time.Second)
	defer cancel()
	if err := j.manager.Reconcile(ctx, limits, os.Getenv("XUI_TC_INTERFACES")); err != nil {
		logger.Warning("tc speed limit: reconciliation failed (will retry):", err)
	}
}

// Watch applies committed edits promptly; the cron job repairs external drift
// even when there are no edits. Both paths share the same serialized manager.
func (j *TCSpeedLimitJob) Watch() {
	j.Run()
	for {
		select {
		case <-j.ctx.Done():
			return
		case <-tclimit.Requests:
			j.Run()
		}
	}
}
