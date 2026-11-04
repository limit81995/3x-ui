//go:build linux

package tclimit

import (
	"context"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

// Opt-in integration test: MUST run in an isolated network namespace, never
// against the host. Example: unshare -Urn env XUI_TC_TEST=1 /tmp/tclimit.test
func TestKernelReconcile(t *testing.T) {
	if os.Getenv("XUI_TC_TEST") != "1" {
		t.Skip("requires an isolated Linux network namespace")
	}
	self, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatal(err)
	}
	host, err := os.Readlink("/proc/1/ns/net")
	if err != nil || self == host {
		t.Fatal("refusing tc integration test in the host network namespace")
	}
	if err := Available(); err != nil {
		t.Fatal(err)
	}
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "tc-limit-test"}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	writes := 0
	m := NewManager()
	m.run = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) > 1 && args[1] == "replace" {
			writes++
		}
		return runTC(ctx, args...)
	}
	ctx := context.Background()
	apply := func(limits []Limit) {
		t.Helper()
		if err := m.Reconcile(ctx, limits, link.Attrs().Name); err != nil {
			for _, dir := range []string{"ingress", "egress"} {
				out, _ := runTC(ctx, "-j", "filter", "show", "dev", link.Attrs().Name, dir)
				t.Log(string(out))
			}
			out, _ := runTC(ctx, "-j", "actions", "ls", "action", "police")
			t.Log(string(out))
			t.Fatal(err)
		}
	}
	read := func() []netlink.Filter {
		t.Helper()
		var all []netlink.Filter
		for _, parent := range []uint32{netlink.HANDLE_MIN_INGRESS, netlink.HANDLE_MIN_EGRESS} {
			filters, err := netlink.FilterList(link, parent)
			if err != nil {
				t.Fatal(err)
			}
			all = append(all, filters...)
		}
		return all
	}
	assertRate := func(want int) {
		t.Helper()
		filters := read()
		if len(filters) != 8 {
			t.Fatalf("got %d filters, want 8", len(filters))
		}
		targets := map[int]int{}
		for _, filter := range filters {
			f := filter.(*netlink.Flower)
			if len(f.Actions) != 1 {
				t.Fatal("unexpected actions")
			}
			redirect, ok := f.Actions[0].(*netlink.MirredAction)
			if !ok {
				t.Fatalf("expected redirect, got %T", f.Actions[0])
			}
			targets[redirect.Ifindex]++
		}
		if len(targets) != 2 {
			t.Fatal(targets)
		}
		for index, count := range targets {
			if count != 4 {
				t.Fatalf("protocols do not share one shaper: %v", targets)
			}
			target, err := netlink.LinkByIndex(index)
			if err != nil {
				t.Fatal(err)
			}
			classes, err := netlink.ClassList(target, rootHandle)
			if err != nil || len(classes) != 1 {
				t.Fatalf("classes=%v err=%v", classes, err)
			}
			if !classEqual(classes[0], Limit{443, want}) {
				t.Fatalf("incorrect HTB class: %+v", classes[0])
			}
		}
	}
	apply([]Limit{{443, 8}})
	assertRate(8)
	initialWrites := writes
	apply([]Limit{{443, 8}})
	if writes != initialWrites {
		t.Fatalf("unchanged kernel state rewrote rules: %d -> %d", initialWrites, writes)
	}

	// Change the live class without touching the saved configuration.
	_, err = runTC(ctx, "class", "change", "dev", "xui-tc-up", "parent", "3a00:", "classid", "3a00:1bb", "htb", "rate", "2mbit", "ceil", "30mbit")
	if err != nil {
		t.Fatal(err)
	}
	apply([]Limit{{443, 8}})
	assertRate(8)
	apply([]Limit{{443, 16}})
	assertRate(16)

	// Delete and alter classifiers, keeping the saved rate unchanged.
	filters := read()
	if err := netlink.FilterDel(filters[0]); err != nil {
		t.Fatal(err)
	}
	f := filters[1].(*netlink.Flower)
	if f.DestPort != 0 {
		f.DestPort = 444
	} else {
		f.SrcPort = 444
	}
	if err := m.replaceFilter(ctx, link.Attrs().Name, f, "xui-tc-up"); err != nil {
		t.Fatal(err)
	}
	apply([]Limit{{443, 16}})
	for _, f := range read() {
		flower := f.(*netlink.Flower)
		if flower.DestPort != 443 && flower.SrcPort != 443 {
			t.Fatalf("filter not repaired: %+v", flower)
		}
	}

	// Simulate process restart and a port edit. Old rules and actions must go.
	m = NewManager()
	apply([]Limit{{8443, 4}})
	for _, f := range read() {
		if f.Attrs().Handle != filterBase|8443 {
			t.Fatal("stale port filter remained")
		}
	}
	apply(nil)
	if len(read()) != 0 {
		t.Fatal("disabling the limit left filters behind")
	}
	out, err := runTC(ctx, "-j", "actions", "ls", "action", "police")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		ids, err := parseActions(out)
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stale actions: %v", ids)
		}
		time.Sleep(20 * time.Millisecond)
		apply(nil)
		out, err = runTC(ctx, "-j", "actions", "ls", "action", "police")
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestParseActionIDs(t *testing.T) {
	ids, err := parseActions([]byte(`[{"total acts":2},{"actions":[{"kind":"police","index":973078415},{"kind":"police","index":42}]}]`))
	if err != nil || !reflect.DeepEqual(ids, map[int]actionState{973078415: {}, 42: {}}) {
		t.Fatalf("%v %v", ids, err)
	}
	if _, err := parseActions([]byte("broken")); err == nil {
		t.Fatal("malformed dump accepted")
	}
}

func TestFilterOwnershipAndDirection(t *testing.T) {
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Index: 10}}
	for _, upload := range []bool{true, false} {
		target := &netlink.Ifb{LinkAttrs: netlink.LinkAttrs{Index: 11}}
		filters := desiredFilters(link, []Limit{{443, 100}}, upload, target)
		if len(filters) != 4 {
			t.Fatal(len(filters))
		}
		for _, f := range filters {
			if !ownedFilter(f) {
				t.Fatal("own filter not recognized")
			}
			if upload && f.DestPort != 443 || !upload && f.SrcPort != 443 {
				t.Fatal("wrong direction")
			}
			if a, ok := f.Actions[0].(*netlink.MirredAction); !ok || a.Ifindex != 11 {
				t.Fatal("invalid redirect")
			}
			f.Handle = 1
			if ownedFilter(f) {
				t.Fatal("unrelated filter claimed")
			}
		}
		for _, f := range desiredFilters(target, []Limit{{443, 100}}, upload, nil) {
			if len(f.Actions) != 0 || f.ClassId != classHandle(443) {
				t.Fatal("incorrect HTB classifier")
			}
		}
	}
	if ownedAction(42) {
		t.Fatal("unrelated legacy action claimed")
	}
}

func TestValidate(t *testing.T) {
	for _, v := range []Limit{{0, 1}, {65536, 1}, {443, -1}, {443, MaxMbps + 1}} {
		if err := Validate(v); err == nil {
			t.Fatalf("accepted %+v", v)
		}
	}
	if err := Validate(Limit{443, MaxMbps}); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Limit{0, 0}); err != nil {
		t.Fatal(err)
	}
}

func TestClassComparisonRejectsChangedEnforcement(t *testing.T) {
	limit := Limit{443, 100}
	want := netlink.NewHtbClass(netlink.ClassAttrs{Handle: classHandle(443), Parent: rootHandle}, netlink.HtbClassAttrs{
		Rate: 100000000, Ceil: 100000000, Buffer: burstBytes(limit), Cbuffer: burstBytes(limit), Quantum: classQuantum(limit),
	})
	if !classEqual(want, limit) {
		t.Fatal("desired class differs")
	}
	for _, change := range []func(*netlink.HtbClass){
		func(h *netlink.HtbClass) { h.Rate++ },
		func(h *netlink.HtbClass) { h.Ceil *= 2 },
		func(h *netlink.HtbClass) { h.Buffer *= 2 },
		func(h *netlink.HtbClass) { h.Cbuffer *= 2 },
		func(h *netlink.HtbClass) { h.Quantum *= 2 },
		func(h *netlink.HtbClass) { h.Parent++ },
	} {
		got := *want
		change(&got)
		if classEqual(&got, limit) {
			t.Fatal("changed enforcement accepted")
		}
	}
}

func TestKernelCoexistenceAndMissingQdisc(t *testing.T) {
	if os.Getenv("XUI_TC_TEST") != "1" {
		t.Skip("requires isolated network namespace")
	}
	self, _ := os.Readlink("/proc/self/ns/net")
	host, err := os.Readlink("/proc/1/ns/net")
	if err != nil || self == host {
		t.Fatal("refusing tc test in host network namespace")
	}
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "tc-coexist"}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	if err := netlink.RouteAdd(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	run := func(args ...string) {
		t.Helper()
		if _, err := runTC(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	run("qdisc", "add", "dev", link.Attrs().Name, "root", "handle", "10:", "fq")
	m := NewManager()
	limits := []Limit{{443, 8}}
	t.Cleanup(func() { _ = m.Reconcile(ctx, nil, "") })
	apply := func(want []Limit) {
		t.Helper()
		if err := m.Reconcile(ctx, want, ""); err != nil {
			t.Fatal(err)
		}
	}
	apply(limits) // auto-select the default-route interface
	run("filter", "add", "dev", link.Attrs().Name, "egress", "protocol", "ip", "pref", "9000", "handle", "1", "flower", "ip_proto", "tcp", "src_port", "22", "action", "pass")
	apply(limits)
	if err := m.Reconcile(ctx, limits, "does-not-exist"); err == nil {
		t.Fatal("missing configured interface accepted")
	}
	apply(nil)
	filters, err := m.kernel.FilterList(link, netlink.HANDLE_MIN_EGRESS)
	if err != nil || len(filters) != 1 || filters[0].Attrs().Priority != 9000 {
		t.Fatalf("unrelated filter lost: %v %v", filters, err)
	}
	qdiscs, err := m.kernel.QdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	foundRoot := false
	for _, q := range qdiscs {
		if q.Type() == "fq" && q.Attrs().Handle == netlink.MakeHandle(0x10, 0) {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Fatal("root qdisc was changed")
	}
	apply(limits)
	run("qdisc", "delete", "dev", link.Attrs().Name, "clsact")
	apply(limits)
	filters, err = m.kernel.FilterList(link, netlink.HANDLE_MIN_EGRESS)
	if err != nil || len(filters) != 4 {
		t.Fatalf("missing clsact not restored: %v %v", filters, err)
	}
}
