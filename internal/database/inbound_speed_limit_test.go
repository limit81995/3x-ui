package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMigrateInboundSpeedLimit(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	legacy := &model.Inbound{Tag: "legacy-tc", Port: 443, Remark: "keep me"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&model.Inbound{}, "speed_limit"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := initModels(); err != nil {
			t.Fatal(err)
		}
		var got model.Inbound
		if err := db.First(&got, legacy.Id).Error; err != nil {
			t.Fatal(err)
		}
		if got.SpeedLimit != 0 || got.Remark != "keep me" {
			t.Fatalf("migration changed legacy inbound: %+v", got)
		}
	}
}
