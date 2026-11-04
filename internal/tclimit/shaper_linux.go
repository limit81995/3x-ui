//go:build linux

package tclimit

import (
	"context"
	"fmt"
	"math"
	"net"
	"strconv"

	"github.com/vishvananda/netlink"
)

type shaperState struct {
	link    netlink.Link
	qdiscs  []netlink.Qdisc
	classes []netlink.Class
	filters []netlink.Filter
}

func classHandle(port int) uint32     { return rootHandle | uint32(port) }
func rateBytes(limit Limit) uint64    { return uint64(limit.Mbps) * 125000 }
func burstBytes(limit Limit) uint32   { return uint32(max(mtu, rateBytes(limit)/100)) }
func classQuantum(limit Limit) uint32 { return uint32(max(1514, min(65536, rateBytes(limit)/1000))) }

func rootEqual(q netlink.Qdisc) bool {
	h, ok := q.(*netlink.Htb)
	return ok && h.Parent == netlink.HANDLE_ROOT && h.Handle == rootHandle && h.Defcls == 0 && h.Rate2Quantum == 10 && h.DirectQlen != nil && *h.DirectQlen == 0
}

func classEqual(c netlink.Class, limit Limit) bool {
	h, ok := c.(*netlink.HtbClass)
	if !ok {
		return false
	}
	// iproute2 and netlink round psched ticks in a slightly different order.
	buffer := netlink.Xmittime(rateBytes(limit), burstBytes(limit))
	tolerance := int64(math.Ceil(netlink.TickInUsec())) + 2
	close := func(v uint32) bool { d := int64(v) - int64(buffer); return d >= -tolerance && d <= tolerance }
	return h.Handle == classHandle(limit.Port) && (h.Parent == rootHandle || h.Parent == netlink.HANDLE_ROOT) &&
		h.Rate == rateBytes(limit) && h.Ceil == rateBytes(limit) && close(h.Buffer) && close(h.Cbuffer) &&
		h.Quantum == classQuantum(limit) && h.Prio == 0 && h.Level == 0
}

func leafEqual(q netlink.Qdisc) bool {
	f, ok := q.(*netlink.FqCodel)
	return ok && f.Limit == 1024 && f.Flows == 1024 && f.Quantum == 1514 && f.MemoryLimit == 4*1024*1024 &&
		f.Target >= 4999 && f.Target <= 5000 && f.Interval >= 99999 && f.Interval <= 100000 && f.ECN == 1
}

func (m *Manager) readShaper(link netlink.Link, qdiscs []netlink.Qdisc) (*shaperState, error) {
	s := &shaperState{link: link, qdiscs: qdiscs}
	var err error
	for _, q := range qdiscs {
		if q.Attrs().Parent == netlink.HANDLE_ROOT && q.Attrs().Handle == rootHandle && q.Type() == "htb" {
			// Restrict the dump to HTB: FQ-CoDel also exposes ephemeral flow
			// classes while traffic is active; those are not HTB drift.
			s.classes, err = m.kernel.ClassList(link, rootHandle)
			if err != nil {
				return nil, fmt.Errorf("read HTB classes on %s: %w", link.Attrs().Name, err)
			}
			s.filters, err = m.kernel.FilterList(link, q.Attrs().Handle)
			if err != nil {
				return nil, fmt.Errorf("read HTB classifiers on %s: %w", link.Attrs().Name, err)
			}
		}
	}
	return s, nil
}

func (m *Manager) prepareShaper(ctx context.Context, s *shaperState, limits []Limit, upload bool) error {
	link := s.link
	name := link.Attrs().Name
	if link.Attrs().MTU != mtu {
		if err := m.kernel.LinkSetMTU(link, mtu); err != nil {
			return err
		}
	}
	if link.Attrs().TxQLen != 1000 {
		if err := m.kernel.LinkSetTxQLen(link, 1000); err != nil {
			return err
		}
	}
	if link.Attrs().Flags&net.FlagUp == 0 {
		if err := m.kernel.LinkSetUp(link); err != nil {
			return err
		}
	}
	validRoot := false
	for _, q := range s.qdiscs {
		if rootEqual(q) {
			validRoot = true
		}
		// The IFB belongs exclusively to the panel; extra ingress/egress
		// hooks could redirect packets around the shaper.
		if q.Type() == "clsact" || q.Type() == "ingress" {
			if _, err := m.run(ctx, "qdisc", "delete", "dev", name, q.Type()); err != nil {
				return err
			}
		}
	}
	for _, c := range s.classes {
		h, ok := c.(*netlink.HtbClass)
		if !ok || h.Level != 0 || (h.Parent != rootHandle && h.Parent != netlink.HANDLE_ROOT) {
			validRoot = false
		}
	}
	if !validRoot {
		// Reset only this owned IFB. Changing a physical root is forbidden.
		for _, q := range s.qdiscs {
			if q.Attrs().Parent == netlink.HANDLE_ROOT && q.Attrs().Handle != 0 {
				if _, err := m.run(ctx, "qdisc", "delete", "dev", name, "root"); err != nil {
					return err
				}
				break
			}
		}
		if _, err := m.run(ctx, "qdisc", "replace", "dev", name, "root", "handle", netlink.HandleStr(rootHandle), "htb", "default", "0", "r2q", "10", "direct_qlen", "0"); err != nil {
			return err
		}
		s.qdiscs, s.classes, s.filters = nil, nil, nil
	}
	classes := map[uint32]netlink.Class{}
	leaves := map[uint32]netlink.Qdisc{}
	current := map[filterKey]netlink.Filter{}
	for _, c := range s.classes {
		classes[c.Attrs().Handle] = c
	}
	for _, q := range s.qdiscs {
		leaves[q.Attrs().Parent] = q
	}
	for _, f := range s.filters {
		current[key(f)] = f
	}
	for _, limit := range limits {
		handle := classHandle(limit.Port)
		if !classEqual(classes[handle], limit) {
			rate := strconv.Itoa(limit.Mbps) + "mbit"
			burst := strconv.FormatUint(uint64(burstBytes(limit)), 10)
			if _, err := m.run(ctx, "class", "replace", "dev", name, "parent", netlink.HandleStr(rootHandle), "classid", netlink.HandleStr(handle), "htb", "rate", rate, "ceil", rate, "burst", burst, "cburst", burst, "quantum", strconv.FormatUint(uint64(classQuantum(limit)), 10), "prio", "0"); err != nil {
				return err
			}
		}
		if !leafEqual(leaves[handle]) {
			// Kernel-assigned leaf handles avoid collisions with rootHandle
			// even when a listening port happens to be 0x3a00.
			if _, err := m.run(ctx, "qdisc", "replace", "dev", name, "parent", netlink.HandleStr(handle), "fq_codel", "limit", "1024", "flows", "1024", "quantum", "1514", "memory_limit", "4194304", "target", "5ms", "interval", "100ms", "ecn"); err != nil {
				return err
			}
		}
	}
	for _, f := range desiredFilters(link, limits, upload, nil) {
		if !filterEqual(current[key(f)], f) {
			if err := m.replaceFilter(ctx, name, f, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) cleanupShaper(ctx context.Context, s *shaperState, limits []Limit) error {
	wantedClasses := map[uint32]bool{}
	wantedFilters := map[filterKey]bool{}
	for _, limit := range limits {
		wantedClasses[classHandle(limit.Port)] = true
	}
	upload := s.link.Attrs().Alias == directions[0].alias
	for _, f := range desiredFilters(s.link, limits, upload, nil) {
		wantedFilters[key(f)] = true
	}
	for _, f := range s.filters {
		if !wantedFilters[key(f)] {
			if err := m.deleteFilter(ctx, s.link.Attrs().Name, f); err != nil {
				return err
			}
		}
	}
	for _, c := range s.classes {
		if !wantedClasses[c.Attrs().Handle] {
			if _, err := m.run(ctx, "class", "delete", "dev", s.link.Attrs().Name, "classid", netlink.HandleStr(c.Attrs().Handle)); err != nil {
				return err
			}
		}
	}
	return nil
}
