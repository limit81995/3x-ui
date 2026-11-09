package runtime

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestWireInboundSpeedLimit(t *testing.T) {
	ib := &model.Inbound{Port: 443, SpeedLimit: 100}
	before := wireInbound(ib, 1)
	if before.Get("speedLimit") != "100" {
		t.Fatal(before)
	}
	ib.SpeedLimit = 0
	after := wireInbound(ib, 1)
	if after.Get("speedLimit") != "0" {
		t.Fatal("clear was omitted")
	}
	if wireFingerprint(before) == wireFingerprint(after) {
		t.Fatal("node reconcile ignores limit changes")
	}
}
