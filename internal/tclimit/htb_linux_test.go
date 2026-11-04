//go:build linux

package tclimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

// A configured port must be backed by two independent, bounded HTB classes,
// rather than two dropping police actions.
func TestKernelHTBClasses(t *testing.T) {
	requireKernelNamespace(t)
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "htb-regression"}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	ctx := context.Background()
	t.Cleanup(func() { _ = m.Reconcile(ctx, nil, link.Attrs().Name) })
	if err := m.Reconcile(ctx, []Limit{{24443, 10}, {24444, 20}}, link.Attrs().Name); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"xui-tc-up", "xui-tc-down"} {
		ifb, err := netlink.LinkByName(name)
		if err != nil {
			t.Fatalf("missing HTB interface %s: %v", name, err)
		}
		if !ownedIFB(ifb) {
			t.Fatalf("IFB ownership missing: type=%s alias=%q", ifb.Type(), ifb.Attrs().Alias)
		}
		classes, err := netlink.ClassList(ifb, rootHandle)
		if err != nil {
			t.Fatal(err)
		}
		if len(classes) != 2 {
			t.Fatalf("%s: got %d classes, want two ports", name, len(classes))
		}
		for _, c := range classes {
			htb, ok := c.(*netlink.HtbClass)
			if !ok {
				t.Fatalf("non-HTB class: %T", c)
			}
			want := uint64(1250000)
			if htb.Handle == netlink.MakeHandle(0x3a00, 24444) {
				want *= 2
			}
			if htb.Rate != want || htb.Ceil != want {
				t.Fatalf("%s: rate=%d ceil=%d, want %d", name, htb.Rate, htb.Ceil, want)
			}
		}
	}
}

func requireKernelNamespace(t *testing.T) {
	t.Helper()
	if os.Getenv("XUI_TC_TEST") != "1" {
		t.Skip("requires isolated Linux network namespace")
	}
	self, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatal(err)
	}
	host, err := os.Readlink("/proc/1/ns/net")
	if err != nil || self == host {
		t.Fatal("refusing tc integration test in host network namespace")
	}
	if err := Available(); err != nil {
		t.Fatal(err)
	}
}

func TestKernelHTBRepairAndOwnership(t *testing.T) {
	requireKernelNamespace(t)
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "htb-repair"}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	ctx := context.Background()
	m := NewManager()
	t.Cleanup(func() { _ = m.Reconcile(ctx, nil, "") })
	apply := func() {
		t.Helper()
		if err := m.Reconcile(ctx, []Limit{{443, 10}, {65535, MaxMbps}, {0x3a00, 1}}, link.Attrs().Name); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) {
		t.Helper()
		if _, err := runTC(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	apply()
	for _, mutate := range []func(){
		func() {
			run("filter", "delete", "dev", "xui-tc-up", "parent", "3a00:")
			run("class", "delete", "dev", "xui-tc-up", "classid", "3a00:1bb")
		},
		func() { run("qdisc", "replace", "dev", "xui-tc-up", "root", "handle", "20:", "fq") },
		func() { run("qdisc", "replace", "dev", "xui-tc-down", "parent", "3a00:1bb", "pfifo", "limit", "1") },
		func() { run("filter", "delete", "dev", "xui-tc-up", "parent", "3a00:", "pref", "48100") },
		func() {
			run("qdisc", "delete", "dev", "xui-tc-up", "root")
			run("qdisc", "add", "dev", "xui-tc-up", "root", "handle", "3a00:", "htb", "default", "1bb", "direct_qlen", "1000")
		},
		func() {
			ifb, _ := netlink.LinkByName("xui-tc-up")
			if err := netlink.LinkSetDown(ifb); err != nil {
				t.Fatal(err)
			}
		},
		func() {
			ifb, _ := netlink.LinkByName("xui-tc-down")
			if err := netlink.LinkDel(ifb); err != nil {
				t.Fatal(err)
			}
		},
		func() {
			ifb, _ := netlink.LinkByName("xui-tc-up")
			if err := netlink.LinkSetName(ifb, "xui-renamed"); err != nil {
				t.Fatal(err)
			}
		},
	} {
		mutate()
		apply()
		// A second read/reconcile must be a no-op, including at extreme rates
		// and ports whose numbers would collide with explicit leaf handles.
		writes := 0
		m.run = func(ctx context.Context, args ...string) ([]byte, error) {
			if args[0] != "-j" {
				writes++
			}
			return runTC(ctx, args...)
		}
		apply()
		m.run = runTC
		if writes != 0 {
			t.Fatalf("repaired state is not stable: %d writes", writes)
		}
		for _, d := range directions {
			ifb, err := netlink.LinkByName(d.name)
			if err != nil {
				t.Fatal(err)
			}
			classes, err := m.kernel.ClassList(ifb, rootHandle)
			if err != nil || len(classes) != 3 {
				t.Fatalf("classes=%v err=%v", classes, err)
			}
			filters, err := m.kernel.FilterList(ifb, rootHandle)
			if err != nil || len(filters) != 12 {
				t.Fatalf("classifiers=%d err=%v", len(filters), err)
			}
		}
	}
	if _, err := netlink.LinkByName("xui-renamed"); err == nil {
		t.Fatal("renamed unused IFB survived")
	}
	if err := m.Reconcile(ctx, nil, ""); err != nil {
		t.Fatal(err)
	}
	// Same name is not sufficient ownership evidence. Refuse takeover.
	foreign := &netlink.Ifb{LinkAttrs: netlink.LinkAttrs{Name: "xui-tc-up"}}
	if err := netlink.LinkAdd(foreign); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(foreign) })
	if err := m.Reconcile(ctx, []Limit{{443, 10}}, link.Attrs().Name); err == nil {
		t.Fatal("foreign IFB was taken over")
	}
	if err := m.Reconcile(ctx, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := netlink.LinkByName("xui-tc-up"); err != nil {
		t.Fatal("foreign IFB was deleted")
	}
	if _, err := netlink.LinkByName("xui-tc-down"); err == nil {
		t.Fatal("partially created HTB despite ownership conflict")
	}
}

func TestKernelMigratesLegacyPolice(t *testing.T) {
	requireKernelNamespace(t)
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "htb-migration"}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	ctx := context.Background()
	run := func(args ...string) {
		t.Helper()
		if _, err := runTC(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	run("qdisc", "add", "dev", link.Attrs().Name, "clsact")
	// One old owned policer, and an unrelated administrator policer.
	for _, index := range []string{"973079415", "42"} {
		run("actions", "add", "action", "police", "index", index, "rate", "10mbit", "burst", "128k", "mtu", "65536", "conform-exceed", "drop/continue", "skip_hw")
	}
	run("filter", "add", "dev", link.Attrs().Name, "ingress", "protocol", "ip", "pref", "48100", "handle", "0x3a0001bb", "flower", "skip_hw", "ip_proto", "tcp", "dst_port", "443", "action", "police", "index", "973079415")
	run("filter", "add", "dev", link.Attrs().Name, "ingress", "protocol", "ip", "pref", "9000", "handle", "42", "flower", "skip_hw", "ip_proto", "tcp", "dst_port", "22", "action", "police", "index", "42")
	m := NewManager()
	t.Cleanup(func() { _ = m.Reconcile(ctx, nil, "") })
	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := m.Reconcile(ctx, []Limit{{443, 10}}, link.Attrs().Name); err != nil {
			t.Fatal(err)
		}
		out, err := runTC(ctx, "-j", "actions", "ls", "action", "police")
		if err != nil {
			t.Fatal(err)
		}
		actions, err := parseActions(out)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := actions[42]; !ok {
			t.Fatal("foreign police action removed")
		}
		if len(actions) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("owned police survived migration: %v", actions)
		}
		time.Sleep(20 * time.Millisecond)
	}
	filters, err := m.kernel.FilterList(link, netlink.HANDLE_MIN_INGRESS)
	if err != nil || len(filters) != 5 {
		t.Fatalf("filters=%d err=%v", len(filters), err)
	}
	for _, f := range filters {
		if ownedFilter(f) {
			if _, ok := f.(*netlink.Flower).Actions[0].(*netlink.MirredAction); !ok {
				t.Fatal("owned filter still polices")
			}
		}
	}
	// Detach/delete the foreign fixture so later tests start clean too.
	run("filter", "delete", "dev", link.Attrs().Name, "ingress", "pref", "9000")
	deadline = time.Now().Add(3 * time.Second)
	for {
		out, err := runTC(ctx, "-j", "actions", "ls", "action", "police")
		if err != nil {
			t.Fatal(err)
		}
		actions, err := parseActions(out)
		if err != nil {
			t.Fatal(err)
		}
		if state, ok := actions[42]; !ok {
			break
		} else if state.bind == 0 {
			run("actions", "delete", "action", "police", "index", "42")
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("foreign fixture still bound")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
