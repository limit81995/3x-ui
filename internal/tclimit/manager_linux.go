//go:build linux

package tclimit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// This namespace is reserved for this panel's rules. Never flush a device or
// replace its root qdisc (which may hold the administrator's fq/HTB setup).
const (
	filterBase uint32 = 0x3a000000
	actionBase        = 0x3a000000
	prefBase   uint16 = 48100
	mtu        uint32 = 65536 // allow GSO packets too
)

type kernel interface {
	LinkList() ([]netlink.Link, error)
	RouteList(netlink.Link, int) ([]netlink.Route, error)
	QdiscList(netlink.Link) ([]netlink.Qdisc, error)
	QdiscAdd(netlink.Qdisc) error
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
			// Normalize at this boundary so comparisons use the kernel meaning.
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

func actionID(port int, upload bool) int {
	id := actionBase | port<<1
	if upload {
		id++
	}
	return id
}

func ownedAction(id int) bool { return id & ^0x1ffff == actionBase }

func ownedFilter(f netlink.Filter) bool {
	a := f.Attrs()
	return a.Priority >= prefBase && a.Priority < prefBase+4 && a.Handle&0xffff0000 == filterBase
}

func police(limit Limit, upload bool) *netlink.PoliceAction {
	p := netlink.NewPoliceAction()
	p.Index = actionID(limit.Port, upload)
	p.Rate = uint32(limit.Mbps) * 125000 // decimal Mbps -> bytes/s
	p.Burst = max(2*mtu, p.Rate/40)      // 25 ms, with room for GSO at low rates
	p.Mtu = mtu
	p.ExceedAction = netlink.TC_POLICE_SHOT
	p.NotExceedAction = netlink.TC_POLICE_UNSPEC // continue through other filters
	return p
}

func actionEqual(got, want *netlink.PoliceAction) bool {
	// Kernel psched time quantization can round the burst by a few bytes.
	diff := int64(got.Burst) - int64(want.Burst)
	return got.Index == want.Index && got.Rate == want.Rate &&
		diff >= -int64(max(16, want.Rate/1000000)) && diff <= int64(max(16, want.Rate/1000000)) &&
		got.Mtu == want.Mtu && got.PeakRate == 0 && got.AvRate == 0 &&
		got.Overhead == 0 && got.Mpu == 0 && got.LinkLayer == 1 &&
		got.ExceedAction == want.ExceedAction && got.NotExceedAction == want.NotExceedAction
}

func desiredFilters(link netlink.Link, limits []Limit) []*netlink.Flower {
	var filters []*netlink.Flower
	for _, limit := range limits {
		for family, eth := range []uint16{unix.ETH_P_IP, unix.ETH_P_IPV6} {
			for transport, proto := range []nl.IPProto{unix.IPPROTO_TCP, unix.IPPROTO_UDP} {
				for _, upload := range []bool{true, false} {
					parent := uint32(netlink.HANDLE_MIN_EGRESS)
					if upload {
						parent = netlink.HANDLE_MIN_INGRESS
					}
					f := &netlink.Flower{
						FilterAttrs: netlink.FilterAttrs{LinkIndex: link.Attrs().Index, Parent: parent,
							Priority: prefBase + uint16(family*2+transport), Protocol: eth, Handle: filterBase | uint32(limit.Port)},
						EthType: eth, IPProto: &proto, SkipHw: true,
						Actions: []netlink.Action{police(limit, upload)},
					}
					if upload {
						f.DestPort = uint16(limit.Port)
					} else {
						f.SrcPort = uint16(limit.Port)
					}
					filters = append(filters, f)
				}
			}
		}
	}
	return filters
}

type filterKey struct {
	link           int
	parent, handle uint32
	priority       uint16
	protocol       uint16
}

func key(f netlink.Filter) filterKey {
	a := f.Attrs()
	return filterKey{a.LinkIndex, a.Parent, a.Handle, a.Priority, a.Protocol}
}

func filterEqual(got netlink.Filter, want *netlink.Flower) bool {
	f, ok := got.(*netlink.Flower)
	if !ok || len(f.Actions) != 1 {
		return false
	}
	a, ok := f.Actions[0].(*netlink.PoliceAction)
	if !ok || a.Index != want.Actions[0].Attrs().Index {
		return false
	}
	x, y := *f, *want
	x.Actions, y.Actions = nil, nil
	// netlink reports an explicit chain zero on some kernels.
	if x.Chain != nil && *x.Chain == 0 {
		x.Chain = nil
	}
	return reflect.DeepEqual(x, y)
}

// Reconcile always reads kernel state, including after a process restart. It
// repairs modified/missing filters and rates and removes obsolete owned rules.
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
	}

	// Read everything before mutating anything: a failed dump must not be
	// mistaken for an empty desired/observed set and cause bulk deletion.
	current := map[filterKey]netlink.Filter{}
	observedActions := map[int]*netlink.PoliceAction{}
	clsact := map[int]bool{}
	for _, link := range links {
		qdiscs, err := m.kernel.QdiscList(link)
		if err != nil {
			return fmt.Errorf("read tc qdisc on %s: %w", link.Attrs().Name, err)
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
				if !ownedFilter(f) {
					continue
				}
				current[key(f)] = f
				if flower, ok := f.(*netlink.Flower); ok {
					for _, a := range flower.Actions {
						if p, ok := a.(*netlink.PoliceAction); ok && ownedAction(p.Index) {
							observedActions[p.Index] = p
						}
					}
				}
			}
		}
	}
	// tc JSON omits police rates in several iproute2 releases. Rates above were
	// read through netlink instead. JSON is only used to discover orphan IDs.
	out, err := m.run(ctx, "-j", "actions", "ls", "action", "police")
	if err != nil {
		return err
	}
	actions, err := parseActions(out)
	if err != nil {
		return err
	}
	wantedActions := map[int]bool{}
	for _, limit := range limits {
		for _, upload := range []bool{true, false} {
			p := police(limit, upload)
			wantedActions[p.Index] = true
			if old := observedActions[p.Index]; old != nil && actions[p.Index].skipHW && actionEqual(old, p) {
				continue
			}
			_, err := m.run(ctx, "actions", "replace", "action", "police", "index", strconv.Itoa(p.Index),
				"rate", strconv.Itoa(limit.Mbps)+"mbit", "burst", strconv.FormatUint(uint64(p.Burst), 10),
				"mtu", strconv.Itoa(int(mtu)), "conform-exceed", "drop/continue", "skip_hw")
			if err != nil {
				return err
			}
		}
	}
	wanted := map[filterKey]bool{}
	for _, link := range links {
		if !selected[link.Attrs().Index] {
			continue
		}
		if !clsact[link.Attrs().Index] {
			q := &netlink.GenericQdisc{QdiscAttrs: netlink.QdiscAttrs{LinkIndex: link.Attrs().Index,
				Handle: netlink.MakeHandle(0xffff, 0), Parent: netlink.HANDLE_CLSACT}, QdiscType: "clsact"}
			if err := m.kernel.QdiscAdd(q); err != nil {
				return fmt.Errorf("add tc clsact on %s: %w", link.Attrs().Name, err)
			}
		}
		for _, f := range desiredFilters(link, limits) {
			k := key(f)
			wanted[k] = true
			if old := current[k]; old != nil && filterEqual(old, f) {
				continue
			}
			if err := m.replaceFilter(ctx, link.Attrs().Name, f); err != nil {
				return fmt.Errorf("apply tc port filter on %s: %w", link.Attrs().Name, err)
			}
		}
	}
	var errs []error
	for k, f := range current {
		if !wanted[k] {
			name := ""
			for _, link := range links {
				if link.Attrs().Index == k.link {
					name = link.Attrs().Name
					break
				}
			}
			if err := m.deleteFilter(ctx, name, f); err != nil {
				errs = append(errs, fmt.Errorf("remove stale tc filter: %w", err))
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	// Unbinding the last filter can release the action as well. Refresh before
	// deleting orphan actions instead of acting on the pre-mutation snapshot.
	out, err = m.run(ctx, "-j", "actions", "ls", "action", "police")
	if err != nil {
		return err
	}
	actions, err = parseActions(out)
	if err != nil {
		return err
	}
	for id, state := range actions {
		// Kernel RCU can defer the final unbind. A later tick collects it.
		if ownedAction(id) && !wantedActions[id] && state.bind == 0 {
			if _, err := m.run(ctx, "actions", "delete", "action", "police", "index", strconv.Itoa(id)); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) deleteFilter(ctx context.Context, name string, f netlink.Filter) error {
	a := f.Attrs()
	direction := "egress"
	if a.Parent == netlink.HANDLE_MIN_INGRESS {
		direction = "ingress"
	}
	_, err := m.run(ctx, "filter", "delete", "dev", name, direction,
		"protocol", fmt.Sprintf("0x%x", a.Protocol), "pref", strconv.Itoa(int(a.Priority)),
		"handle", fmt.Sprintf("0x%x", a.Handle), f.Type())
	return err
}

func (m *Manager) replaceFilter(ctx context.Context, name string, f *netlink.Flower) error {
	direction, portKey, port := "egress", "src_port", f.SrcPort
	if f.Parent == netlink.HANDLE_MIN_INGRESS {
		direction, portKey, port = "ingress", "dst_port", f.DestPort
	}
	family, transport := "ip", "tcp"
	if f.EthType == unix.ETH_P_IPV6 {
		family = "ipv6"
	}
	if *f.IPProto == unix.IPPROTO_UDP {
		transport = "udp"
	}
	_, err := m.run(ctx, "filter", "replace", "dev", name, direction, "protocol", family,
		"pref", strconv.Itoa(int(f.Priority)), "handle", fmt.Sprintf("0x%x", f.Handle),
		"flower", "skip_hw", "ip_proto", transport, portKey, strconv.Itoa(int(port)),
		"action", "police", "index", strconv.Itoa(f.Actions[0].Attrs().Index))
	return err
}

func (m *Manager) selectLinks(links []netlink.Link, configured string) (map[int]bool, error) {
	selected := map[int]bool{}
	if strings.TrimSpace(configured) != "" {
		for _, name := range strings.Split(configured, ",") {
			name = strings.TrimSpace(name)
			found := false
			for _, link := range links {
				if link.Attrs().Name == name && link.Attrs().Flags&net.FlagLoopback == 0 {
					selected[link.Attrs().Index], found = true, true
				}
			}
			if !found {
				return nil, fmt.Errorf("XUI_TC_INTERFACES: interface %q is missing or loopback", name)
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
			if link.Attrs().Flags&net.FlagLoopback != 0 {
				delete(selected, link.Attrs().Index)
			}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("tc speedLimit: no default-route interface; set XUI_TC_INTERFACES")
	}
	return selected, nil
}

type actionState struct {
	bind   int
	skipHW bool
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
					ids[int(index)] = actionState{bind: int(bind), skipHW: v["skip_hw"] == true}
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
