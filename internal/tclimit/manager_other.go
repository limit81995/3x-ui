//go:build !linux

package tclimit

import (
	"context"
	"fmt"
)

type Manager struct{}

func NewManager() *Manager { return &Manager{} }
func Available() error     { return fmt.Errorf("tc speedLimit requires Linux") }
func (*Manager) Reconcile(_ context.Context, limits []Limit, _ string) error {
	if len(limits) > 0 {
		return Available()
	}
	return nil
}
