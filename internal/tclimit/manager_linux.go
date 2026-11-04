//go:build linux

package tclimit

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// The physical interface's root qdisc and unrelated clsact filters stay intact.
// HTB roots live exclusively on the two panel-owned IFBs.
const (
	filterBase uint32 = 0x3a000000
	rootHandle uint32 = 0x3a000000
	prefBase   uint16 = 48100
	mtu               = 65536 // GSO frames must fit on the IFBs
)

var directions = [...]struct {
	name, alias string
	upload      bool
}{
	{"xui-tc-up", "3x-ui:tc:htb:upload:v1", true},
	{"xui-tc-down", "3x-ui:tc:htb:download:v1", false},
}

type kernel interface {
	LinkList() ([]netlink.Link, error)
	LinkAdd(netlink.Link) error
	LinkDel(netlink.Link) error
	LinkSetUp(netlink.Link) error
	LinkSetAlias(netlink.Link, string) error
	LinkSetMTU(netlink.Link, int) error
	LinkSetTxQLen(netlink.Link, int) error
	RouteList(netlink.Link, int) ([]netlink.Route, error)
	QdiscList(netlink.Link) ([]netlink.Qdisc, error)
	ClassList(netlink.Link, uint32) ([]netlink.Class, error)
	FilterList(netlink.Link, uint32) ([]netlink.Filter, error)
}

type Manager struct {
	mu     sync.Mutex
	kernel kernel
	run    func(context.Context, ...string) ([]byte, error)
}

type linuxKernel struct{ *netlink.Handle }

func (k *linuxKernel) FilterList(link netlink.Link, parent uint32) ([]netlink.Filter, error) {
	filters, err := k.Handle.FilterList(link, parent)
	for _, filter := range filters {
		if f, ok := filter.(*netlink.Flower); ok {
			// The pinned netlink v1.3.1 decoder swaps SKIP_HW and SKIP_SW.
			f.SkipHw, f.SkipSw = f.SkipSw, f.SkipHw
		}
	}
	return filters, err
}

func NewManager() *Manager { return &Manager{kernel: &linuxKernel{&netlink.Handle{}}, run: runTC} }

func Available() error {
	if _, err := exec.LookPath("tc"); err != nil {
		return fmt.Errorf("tc speedLimit requires iproute2-tc (Alpine) or iproute2 (Debian/Ubuntu): %w", err)
	}
	// Fail at save time when a container has no NET_ADMIN, even if it is root.
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "CapEff:") {
			caps, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "CapEff:")), 16, 64)
			if err == nil && caps&(1<<unix.CAP_NET_ADMIN) != 0 {
				return nil
			}
			return fmt.Errorf("tc speedLimit requires CAP_NET_ADMIN; add cap_add: [NET_ADMIN] to the container")
		}
	}
	return fmt.Errorf("cannot determine CAP_NET_ADMIN for tc speedLimit")
}

func ownedIFB(link netlink.Link) bool {
	if link.Type() != "ifb" {
		return false
	}
	for _, d := range directions {
		if link.Attrs().Alias == d.alias {
			return true
		}
	}
	return false
}

// Reconcile reads live kernel state, preparing HTB before switching redirects
// and detaching obsolete physical filters before deleting their targets.
func (m *Manager) Reconcile(ctx context.Context, limits []Limit, interfaces string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rates := map[int]int{}
	for _, limit := range limits {
		if err := Validate(limit); err != nil {
			return err
		}
		if limit.Mbps == 0 {
			continue
		}
		if old, ok := rates[limit.Port]; ok && old != limit.Mbps {
			return fmt.Errorf("conflicting tc limits for port %d", limit.Port)
		}
		rates[limit.Port] = limit.Mbps
	}
	limits = nil
	for port, rate := range rates {
		limits = append(limits, Limit{port, rate})
	}
	slices.SortFunc(limits, func(a, b Limit) int { return a.Port - b.Port })
	links, err := m.kernel.LinkList()
	if err != nil {
		return fmt.Errorf("list tc interfaces: %w", err)
	}
	selected := map[int]bool{}
	if len(limits) > 0 {
		selected, err = m.selectLinks(links, interfaces)
		if err != nil {
			return err
		}
		for _, link := range links {
			for _, d := range directions {
				if link.Attrs().Name == d.name && (link.Type() != "ifb" || link.Attrs().Alias != d.alias) {
					return fmt.Errorf("tc speedLimit: interface %s already exists and is not owned by this panel", d.name)
				}
			}
		}
	}

	// Snapshot before mutations. A failed dump must never look like an empty
	// observed set and trigger destructive cleanup.
	current := map[filterKey]netlink.Filter{}
	clsact := map[int]bool{}
	shapers := map[string]*shaperState{}
	for _, link := range links {
		qdiscs, err := m.kernel.QdiscList(link)
		if err != nil {
			return fmt.Errorf("read tc qdisc on %s: %w", link.Attrs().Name, err)
		}
		if ownedIFB(link) {
			state, err := m.readShaper(link, qdiscs)
			if err != nil {
				return err
			}
			shapers[link.Attrs().Name] = state
			continue
		}
		hasIngress := false
		for _, q := range qdiscs {
			if q.Type() == "clsact" {
				clsact[link.Attrs().Index] = true
			}
			if q.Type() == "ingress" {
				hasIngress = true
			}
		}
		if selected[link.Attrs().Index] && hasIngress && !clsact[link.Attrs().Index] {
			return fmt.Errorf("%s has an ingress qdisc; tc speedLimit requires clsact (existing qdisc left intact)", link.Attrs().Name)
		}
		if !clsact[link.Attrs().Index] && !hasIngress {
			continue
		}
		for _, parent := range []uint32{netlink.HANDLE_MIN_INGRESS, netlink.HANDLE_MIN_EGRESS} {
			if hasIngress && parent == netlink.HANDLE_MIN_EGRESS {
				continue
			}
			filters, err := m.kernel.FilterList(link, parent)
			if err != nil {
				return fmt.Errorf("read tc filters on %s: %w", link.Attrs().Name, err)
			}
			for _, f := range filters {
				if ownedFilter(f) {
					current[key(f)] = f
				}
			}
		}
	}

	targets := map[bool]netlink.Link{}
	if len(limits) > 0 {
		for _, d := range directions {
			state := shapers[d.name]
			if state == nil {
				link := &netlink.Ifb{LinkAttrs: netlink.LinkAttrs{Name: d.name, Alias: d.alias, MTU: mtu, TxQLen: 1000}}
				if err := m.kernel.LinkAdd(link); err != nil {
					return fmt.Errorf("create tc IFB %s (kernel IFB support required): %w", d.name, err)
				}
				// Some kernels ignore IFLA_IFALIAS during link creation.
				if err := m.kernel.LinkSetAlias(link, d.alias); err != nil {
					_ = m.kernel.LinkDel(link)
					return fmt.Errorf("mark ownership of tc IFB %s: %w", d.name, err)
				}
				state = &shaperState{link: link}
				shapers[d.name] = state
			}
			if err := m.prepareShaper(ctx, state, limits, d.upload); err != nil {
				return fmt.Errorf("prepare HTB on %s: %w", d.name, err)
			}
			targets[d.upload] = state.link
		}
	}
	wanted := map[filterKey]bool{}
	for _, link := range links {
		if !selected[link.Attrs().Index] {
			continue
		}
		if !clsact[link.Attrs().Index] {
			if _, err := m.run(ctx, "qdisc", "add", "dev", link.Attrs().Name, "clsact"); err != nil {
				return err
			}
		}
		for _, d := range directions {
			for _, f := range desiredFilters(link, limits, d.upload, targets[d.upload]) {
				k := key(f)
				wanted[k] = true
				if old := current[k]; old != nil && filterEqual(old, f) {
					continue
				}
				if err := m.replaceFilter(ctx, link.Attrs().Name, f, targets[d.upload].Attrs().Name); err != nil {
					return err
				}
			}
		}
	}
	for k, f := range current {
		if wanted[k] {
			continue
		}
		for _, link := range links {
			if link.Attrs().Index == k.link {
				if err := m.deleteFilter(ctx, link.Attrs().Name, f); err != nil {
					return err
				}
				break
			}
		}
	}
	for name, state := range shapers {
		var active bool
		for _, target := range targets {
			active = active || target.Attrs().Name == name
		}
		if !active {
			if err := m.kernel.LinkDel(state.link); err != nil {
				return fmt.Errorf("delete unused tc IFB %s: %w", name, err)
			}
		} else if err := m.cleanupShaper(ctx, state, limits); err != nil {
			return err
		}
	}
	return m.cleanupLegacyActions(ctx)
}

func (m *Manager) selectLinks(links []netlink.Link, configured string) (map[int]bool, error) {
	selected := map[int]bool{}
	if strings.TrimSpace(configured) != "" {
		for _, name := range strings.Split(configured, ",") {
			name = strings.TrimSpace(name)
			found := false
			for _, link := range links {
				if link.Attrs().Name == name && link.Attrs().Flags&net.FlagLoopback == 0 && link.Type() != "ifb" {
					selected[link.Attrs().Index], found = true, true
				}
			}
			if !found {
				return nil, fmt.Errorf("XUI_TC_INTERFACES: interface %q is missing, loopback or IFB", name)
			}
		}
	} else {
		routes, err := m.kernel.RouteList(nil, netlink.FAMILY_ALL)
		if err != nil {
			return nil, err
		}
		for _, route := range routes {
			if route.Dst != nil {
				ones, _ := route.Dst.Mask.Size()
				if ones != 0 {
					continue
				}
			}
			if route.LinkIndex > 0 {
				selected[route.LinkIndex] = true
			}
			for _, next := range route.MultiPath {
				if next.LinkIndex > 0 {
					selected[next.LinkIndex] = true
				}
			}
		}
		for _, link := range links {
			if link.Attrs().Flags&net.FlagLoopback != 0 || link.Type() == "ifb" {
				delete(selected, link.Attrs().Index)
			}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("tc speedLimit: no default-route interface; set XUI_TC_INTERFACES")
	}
	return selected, nil
}
