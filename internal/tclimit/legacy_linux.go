//go:build linux

package tclimit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Only migration cleanup uses police. No new policers are created.
const actionBase = 0x3a000000

func ownedAction(id int) bool { return id & ^0x1ffff == actionBase }

func (m *Manager) cleanupLegacyActions(ctx context.Context) error {
	// A fresh HTB installation does not need the act_police kernel module.
	if _, err := os.Stat("/sys/module/act_police"); os.IsNotExist(err) {
		return nil
	}
	out, err := m.run(ctx, "-j", "actions", "ls", "action", "police")
	if err != nil {
		return err
	}
	actions, err := parseActions(out)
	if err != nil {
		return err
	}
	var errs []error
	for id, state := range actions {
		// RCU can delay the final unbind; a later tick collects those actions.
		if ownedAction(id) && state.bind == 0 {
			if _, err := m.run(ctx, "actions", "delete", "action", "police", "index", strconv.Itoa(id)); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

type actionState struct {
	bind int
}

func parseActions(data []byte) (map[int]actionState, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("read tc police action IDs: %w", err)
	}
	ids := map[int]actionState{}
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			if v["kind"] == "police" {
				if index, ok := v["index"].(float64); ok {
					bind, _ := v["bind"].(float64)
					ids[int(index)] = actionState{bind: int(bind)}
				}
			}
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(raw)
	return ids, nil
}
