//go:build linux

package tclimit

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

func ownedFilter(f netlink.Filter) bool {
	a := f.Attrs()
	return a.Priority >= prefBase && a.Priority < prefBase+4 && a.Handle&0xffff0000 == filterBase
}

// A nil target builds IFB classifiers. Otherwise build physical redirects.
// All four protocol combinations share one HTB class for this port/direction.
func desiredFilters(link netlink.Link, limits []Limit, upload bool, target netlink.Link) []*netlink.Flower {
	var filters []*netlink.Flower
	parent := rootHandle
	if target != nil {
		parent = netlink.HANDLE_MIN_EGRESS
		if upload {
			parent = netlink.HANDLE_MIN_INGRESS
		}
	}
	for _, limit := range limits {
		for family, eth := range []uint16{unix.ETH_P_IP, unix.ETH_P_IPV6} {
			for transport, proto := range []nl.IPProto{unix.IPPROTO_TCP, unix.IPPROTO_UDP} {
				f := &netlink.Flower{
					FilterAttrs: netlink.FilterAttrs{LinkIndex: link.Attrs().Index, Parent: parent,
						Priority: prefBase + uint16(family*2+transport), Protocol: eth, Handle: filterBase | uint32(limit.Port)},
					EthType: eth, IPProto: &proto, SkipHw: true,
				}
				if upload {
					f.DestPort = uint16(limit.Port)
				} else {
					f.SrcPort = uint16(limit.Port)
				}
				if target == nil {
					f.ClassId = classHandle(limit.Port)
				} else {
					f.Actions = []netlink.Action{netlink.NewMirredAction(target.Attrs().Index)}
				}
				filters = append(filters, f)
			}
		}
	}
	return filters
}

type filterKey struct {
	link                  int
	parent, handle, chain uint32
	priority, protocol    uint16
}

func key(f netlink.Filter) filterKey {
	a := f.Attrs()
	var chain uint32
	if a.Chain != nil {
		chain = *a.Chain
	}
	return filterKey{a.LinkIndex, a.Parent, a.Handle, chain, a.Priority, a.Protocol}
}

func filterEqual(got netlink.Filter, want *netlink.Flower) bool {
	f, ok := got.(*netlink.Flower)
	if !ok || len(f.Actions) != len(want.Actions) {
		return false
	}
	if len(want.Actions) > 0 {
		a, ok := f.Actions[0].(*netlink.MirredAction)
		b := want.Actions[0].(*netlink.MirredAction)
		if !ok || a.Ifindex != b.Ifindex || a.MirredAction != b.MirredAction || a.Action != b.Action {
			return false
		}
	}
	x, y := *f, *want
	x.Actions, y.Actions = nil, nil
	if x.Chain != nil && *x.Chain == 0 {
		x.Chain = nil
	}
	return reflect.DeepEqual(x, y)
}

func filterParent(parent uint32) []string {
	switch parent {
	case netlink.HANDLE_MIN_INGRESS:
		return []string{"ingress"}
	case netlink.HANDLE_MIN_EGRESS:
		return []string{"egress"}
	default:
		return []string{"parent", netlink.HandleStr(parent)}
	}
}

func (m *Manager) deleteFilter(ctx context.Context, name string, f netlink.Filter) error {
	a := f.Attrs()
	args := append([]string{"filter", "delete", "dev", name}, filterParent(a.Parent)...)
	if a.Chain != nil {
		args = append(args, "chain", strconv.FormatUint(uint64(*a.Chain), 10))
	}
	args = append(args, "protocol", fmt.Sprintf("0x%x", a.Protocol), "pref", strconv.Itoa(int(a.Priority)), "handle", fmt.Sprintf("0x%x", a.Handle), f.Type())
	_, err := m.run(ctx, args...)
	return err
}

func (m *Manager) replaceFilter(ctx context.Context, name string, f *netlink.Flower, targetName string) error {
	portKey, port := "src_port", f.SrcPort
	if f.DestPort != 0 {
		portKey, port = "dst_port", f.DestPort
	}
	family, transport := "ip", "tcp"
	if f.EthType == unix.ETH_P_IPV6 {
		family = "ipv6"
	}
	if *f.IPProto == unix.IPPROTO_UDP {
		transport = "udp"
	}
	args := append([]string{"filter", "replace", "dev", name}, filterParent(f.Parent)...)
	args = append(args, "protocol", family, "pref", strconv.Itoa(int(f.Priority)), "handle", fmt.Sprintf("0x%x", f.Handle), "flower", "skip_hw", "ip_proto", transport, portKey, strconv.Itoa(int(port)))
	if f.ClassId != 0 {
		args = append(args, "classid", netlink.HandleStr(f.ClassId))
	} else {
		args = append(args, "action", "mirred", "egress", "redirect", "dev", targetName)
	}
	_, err := m.run(ctx, args...)
	return err
}
