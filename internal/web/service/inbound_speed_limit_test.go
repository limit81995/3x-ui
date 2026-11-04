package service

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tclimit"
)

func TestInboundSpeedLimitPersistenceAndClear(t *testing.T) {
	setupConflictDB(t)
	previous := tcAvailable
	tcAvailable = func() error { return nil }
	t.Cleanup(func() { tcAvailable = previous })
	svc := &InboundService{}
	ib := makeInboundWithSubSortIndex("in-17001-tcp", 17001, 1)
	ib.SpeedLimit = 100
	created, _, err := svc.AddInbound(ib)
	if err != nil {
		t.Fatal(err)
	}
	for _, rate := range []int{100, 50, 0} {
		edit := *created
		edit.SpeedLimit = rate
		got, _, err := svc.UpdateInbound(&edit)
		if err != nil {
			t.Fatal(err)
		}
		if got.SpeedLimit != rate {
			t.Fatalf("response rate %d != %d", got.SpeedLimit, rate)
		}
		var stored model.Inbound
		if err := database.GetDB().First(&stored, created.Id).Error; err != nil {
			t.Fatal(err)
		}
		if stored.SpeedLimit != rate {
			t.Fatalf("stored rate %d != %d", stored.SpeedLimit, rate)
		}
		raw, err := json.Marshal(&stored)
		if err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err)
		}
		if response["speedLimit"] != float64(rate) {
			t.Fatal(response)
		}
	}
	// The controller is not the only write path: service/import callers must
	// reject invalid values before persisting as well.
	edit := *created
	edit.SpeedLimit = -1
	if _, _, err := svc.UpdateInbound(&edit); err == nil {
		t.Fatal("negative speedLimit accepted")
	}
}

func TestInboundSpeedLimitMissingCapabilities(t *testing.T) {
	setupConflictDB(t)
	previous := tcAvailable
	tcAvailable = func() error { return errors.New("missing CAP_NET_ADMIN") }
	t.Cleanup(func() { tcAvailable = previous })
	ib := makeInboundWithSubSortIndex("in-17002-tcp", 17002, 1)
	ib.SpeedLimit = 10
	if _, _, err := (&InboundService{}).AddInbound(ib); err == nil {
		t.Fatal("unusable limit accepted")
	}
	var count int64
	database.GetDB().Model(&model.Inbound{}).Count(&count)
	if count != 0 {
		t.Fatal("failed preflight persisted the inbound")
	}
	ib.SpeedLimit = 0
	if err := validateInboundSpeedLimit(ib); err != nil {
		t.Fatal(err)
	}
	node := 5
	ib.NodeID, ib.SpeedLimit = &node, 10
	if err := validateInboundSpeedLimit(ib); err != nil {
		t.Fatal("node limit validated against the master host", err)
	}
	ib.Listen = "127.0.0.1"
	if err := validateInboundSpeedLimit(ib); err == nil {
		t.Fatal("ineffective loopback limit accepted")
	}
}

func TestDesiredTCSpeedLimitsOnlyActiveLocalInbounds(t *testing.T) {
	setupConflictDB(t)
	node := 12
	rows := []model.Inbound{
		{Tag: "active", Port: 18001, Enable: true, SpeedLimit: 10},
		{Tag: "remote", Port: 18002, Enable: true, SpeedLimit: 20, NodeID: &node},
		{Tag: "disabled", Port: 18003, SpeedLimit: 30},
		{Tag: "unlimited", Port: 18004, Enable: true},
		{Tag: "expired", Port: 18005, Enable: true, SpeedLimit: 50, ExpiryTime: time.Now().Add(-time.Hour).UnixMilli()},
		{Tag: "exhausted", Port: 18006, Enable: true, SpeedLimit: 60, Total: 100, Up: 50, Down: 50},
	}
	if err := database.GetDB().Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	got, err := (&InboundService{}).DesiredTCSpeedLimits()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []tclimit.Limit{{Port: 18001, Mbps: 10}}) {
		t.Fatal(got)
	}
	if err := database.GetDB().Delete(&rows[0]).Error; err != nil {
		t.Fatal(err)
	}
	got, err = (&InboundService{}).DesiredTCSpeedLimits()
	if err != nil || len(got) != 0 {
		t.Fatalf("deleted inbound retained: %v %v", got, err)
	}
}

func TestPortSpeedLimitConflictScope(t *testing.T) {
	setupConflictDB(t)
	stored := model.Inbound{Tag: "shared", Port: 19001, Listen: "192.0.2.1", Enable: true, SpeedLimit: 100}
	if err := database.GetDB().Create(&stored).Error; err != nil {
		t.Fatal(err)
	}
	candidate := model.Inbound{Port: 19001, Listen: "192.0.2.2", Enable: true, SpeedLimit: 50}
	if err := checkInboundSpeedLimitTx(database.GetDB(), &candidate, 0); err == nil {
		t.Fatal("ambiguous shared-port cap accepted")
	}
	candidate.SpeedLimit = 100
	if err := checkInboundSpeedLimitTx(database.GetDB(), &candidate, 0); err != nil {
		t.Fatal(err)
	}
	candidate.SpeedLimit = 0
	if err := checkInboundSpeedLimitTx(database.GetDB(), &candidate, 0); err == nil {
		t.Fatal("unlimited row would actually be capped")
	}
	node := 2
	candidate.NodeID = &node
	if err := checkInboundSpeedLimitTx(database.GetDB(), &candidate, 0); err != nil {
		t.Fatal("cross-node conflict", err)
	}
}
