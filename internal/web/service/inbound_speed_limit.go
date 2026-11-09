package service

import (
	"fmt"
	"net"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tclimit"
)

var tcAvailable = tclimit.Available

func validateInboundSpeedLimit(inbound *model.Inbound) error {
	if err := tclimit.Validate(tclimit.Limit{Port: inbound.Port, Mbps: inbound.SpeedLimit}); err != nil {
		return err
	}
	if inbound.SpeedLimit == 0 {
		return nil
	}
	listen := strings.Trim(strings.TrimSpace(inbound.Listen), "[]")
	if inbound.Protocol == "tun" || strings.HasPrefix(listen, "/") || strings.HasPrefix(listen, "@") {
		return fmt.Errorf("tc speedLimit requires a TCP/UDP listener")
	}
	if ip := net.ParseIP(listen); ip != nil && ip.IsLoopback() {
		return fmt.Errorf("tc speedLimit cannot limit a loopback listener; set the limit on its public inbound")
	}
	if inbound.NodeID == nil {
		return tcAvailable()
	}
	return nil // the hosting node validates its own capabilities when pushed
}

// A port-wide policer cannot distinguish listeners on different addresses or
// transports. Require their configured rates to agree instead of silently
// choosing one row and displaying a different rate for another.
func checkInboundSpeedLimitTx(db *gorm.DB, inbound *model.Inbound, ignoreID int) error {
	if !inbound.Enable {
		return nil
	}
	q := db.Model(&model.Inbound{}).Where("id <> ? AND port = ? AND enable = ? AND speed_limit <> ?", ignoreID, inbound.Port, true, inbound.SpeedLimit)
	if inbound.NodeID == nil {
		q = q.Where("node_id IS NULL OR node_id = 0")
	} else {
		q = q.Where("node_id = ?", *inbound.NodeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("enabled inbounds sharing port %d on the same node must have the same speedLimit", inbound.Port)
	}
	return nil
}

func (*InboundService) DesiredTCSpeedLimits() ([]tclimit.Limit, error) {
	var inbounds []model.Inbound
	if err := database.GetDB().Select("port, speed_limit").
		Where("(node_id IS NULL OR node_id = 0) AND enable = ? AND speed_limit > 0", true).
		Where("expiry_time = 0 OR expiry_time > ?", time.Now().UnixMilli()).
		Where("total = 0 OR up + down < total").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	limits := make([]tclimit.Limit, 0, len(inbounds))
	for _, inbound := range inbounds {
		limits = append(limits, tclimit.Limit{Port: inbound.Port, Mbps: inbound.SpeedLimit})
	}
	return limits, nil
}
