//go:build linux

package tclimit

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// Real TCP across two isolated namespaces must sustain each directional cap
// while reconciliation preserves active classes and queues.
func TestKernelTCPThroughput(t *testing.T) {
	requireKernelNamespace(t)
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	peerNS, createErr := netns.New()
	restoreErr := netns.Set(original)
	original.Close()
	if restoreErr != nil {
		t.Fatalf("restore test namespace: %v", restoreErr)
	}
	runtime.UnlockOSThread()
	if createErr != nil {
		t.Fatal(createErr)
	}
	t.Cleanup(func() { peerNS.Close() })
	peerHandle, err := netlink.NewHandleAt(peerNS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(peerHandle.Close)
	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: "htb-tcp"}, PeerName: "htb-tcp-peer", PeerNamespace: netlink.NsFd(peerNS)}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	addr, _ := netlink.ParseAddr("198.18.1.1/24")
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	peer, err := peerHandle.LinkByName(link.PeerName)
	if err != nil {
		t.Fatal(err)
	}
	addr, _ = netlink.ParseAddr("198.18.1.2/24")
	if err := peerHandle.AddrAdd(peer, addr); err != nil {
		t.Fatal(err)
	}
	if err := peerHandle.LinkSetUp(peer); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("198.18.1.1"), Port: 24443})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	m := NewManager()
	ctx := context.Background()
	t.Cleanup(func() { _ = m.Reconcile(ctx, nil, link.Attrs().Name) })
	apply := func() {
		t.Helper()
		if err := m.Reconcile(ctx, []Limit{{24443, 10}}, link.Attrs().Name); err != nil {
			t.Fatal(err)
		}
	}
	apply()

	// An established socket retains its namespace after the thread is restored.
	runtime.LockOSThread()
	original, err = netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	err = netns.Set(peerNS)
	var client *net.TCPConn
	if err == nil {
		client, err = net.DialTCP("tcp4", nil, listener.Addr().(*net.TCPAddr))
	}
	restoreErr = netns.Set(original)
	original.Close()
	if restoreErr != nil {
		t.Fatalf("restore test namespace: %v", restoreErr)
	}
	runtime.UnlockOSThread()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	server, err := listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var uploaded, downloaded atomic.Uint64
	var wg sync.WaitGroup
	for i, conn := range []*net.TCPConn{server, client} {
		conn.SetDeadline(time.Now().Add(12 * time.Second))
		counter := &uploaded
		if i == 1 {
			counter = &downloaded
		}
		wg.Add(2)
		go func() {
			defer wg.Done()
			buf := make([]byte, 32768)
			for {
				if _, err := conn.Write(buf); err != nil {
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			buf := make([]byte, 65536)
			for {
				n, err := conn.Read(buf)
				counter.Add(uint64(n))
				if err != nil {
					return
				}
			}
		}()
	}
	defer func() { server.Close(); client.Close(); wg.Wait() }()
	time.Sleep(2 * time.Second) // allow the initial burst and TCP startup to settle
	start := time.Now()
	up, down := uploaded.Load(), downloaded.Load()
	writes := 0
	m.run = func(ctx context.Context, args ...string) ([]byte, error) {
		if args[0] != "-j" {
			writes++
		}
		return runTC(ctx, args...)
	}
	for range 4 {
		apply()
		time.Sleep(time.Second)
	}
	elapsed := time.Since(start).Seconds()
	for _, result := range []struct {
		name  string
		bytes uint64
	}{{"upload", uploaded.Load() - up}, {"download", downloaded.Load() - down}} {
		mbps := float64(result.bytes) * 8 / 1e6 / elapsed
		t.Logf("real TCP %s: %.2f Mbps at a 10 Mbps cap", result.name, mbps)
		if mbps < 8 || mbps > 10.8 {
			t.Fatalf("unexpected %s throughput: %.2f Mbps", result.name, mbps)
		}
	}
	if writes != 0 {
		t.Fatalf("reconcile reset active queues/classes: %d writes", writes)
	}
}
