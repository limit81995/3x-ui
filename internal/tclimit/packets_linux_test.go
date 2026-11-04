//go:build linux

package tclimit

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Flood both directions concurrently with alternating IPv4/IPv6, TCP/UDP
// frames. A single bucket per direction must cap their combined throughput.
func TestKernelPacketsShareDirectionalLimits(t *testing.T) {
	if os.Getenv("XUI_TC_TEST") != "1" {
		t.Skip("requires isolated network namespace")
	}
	self, _ := os.Readlink("/proc/self/ns/net")
	host, err := os.Readlink("/proc/1/ns/net")
	if err != nil || self == host {
		t.Fatal("refusing packet test in host network namespace")
	}
	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: "tc-packet-test"}, PeerName: "tc-packet-peer"}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	peer, err := netlink.LinkByName(link.PeerName)
	if err != nil {
		t.Fatal(err)
	}
	for _, dev := range []netlink.Link{link, peer} {
		if err := netlink.LinkSetUp(dev); err != nil {
			t.Fatal(err)
		}
	}
	m := NewManager()
	if err := m.Reconcile(context.Background(), []Limit{{443, 8}}, link.Attrs().Name); err != nil {
		t.Fatal(err)
	}
	const frameSize = 1254
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	start := time.Now()
	for _, upload := range []bool{true, false} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sender, receiver := netlink.Link(link), peer
			if upload {
				sender, receiver = peer, link
			}
			fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
			if err != nil {
				errCh <- err
				return
			}
			defer unix.Close(fd)
			frames := make([][]byte, 4)
			for i := range frames {
				frames[i] = packetFrame(frameSize, i >= 2, i%2 == 1, upload)
				copy(frames[i][:6], receiver.Attrs().HardwareAddr)
				copy(frames[i][6:12], sender.Attrs().HardwareAddr)
			}
			address := &unix.SockaddrLinklayer{Ifindex: sender.Attrs().Index, Halen: 6}
			for i := 0; time.Since(start) < time.Second; i++ {
				frame := frames[i%len(frames)]
				address.Protocol = htons(binary.BigEndian.Uint16(frame[12:14]))
				if err := unix.Sendto(fd, frame, 0, address); err != nil && !errors.Is(err, unix.ENOBUFS) {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	elapsed := time.Since(start).Seconds()
	for _, parent := range []uint32{netlink.HANDLE_MIN_INGRESS, netlink.HANDLE_MIN_EGRESS} {
		filters, err := m.kernel.FilterList(link, parent)
		if err != nil {
			t.Fatal(err)
		}
		if len(filters) != 4 {
			t.Fatal(len(filters))
		}
		p := filters[0].(*netlink.Flower).Actions[0].(*netlink.PoliceAction)
		if p.Statistics == nil || p.Statistics.Basic == nil || p.Statistics.Queue == nil {
			t.Fatal("missing packet counters")
		}
		stats := p.Statistics
		passed := uint64(stats.Basic.Packets) - uint64(stats.Queue.Drops)
		mbps := float64(passed*frameSize) * 8 / 1e6 / elapsed
		t.Logf("parent=%x offered=%d dropped=%d passed=%.2f Mbps", parent, stats.Basic.Packets, stats.Queue.Drops, mbps)
		// The initial burst can add about 1 Mbps over this one-second sample.
		if mbps < 4 || mbps > 10 || stats.Queue.Drops == 0 {
			t.Fatalf("directional cap ineffective: %.2f Mbps", mbps)
		}
	}
}

func htons(v uint16) uint16 { return v<<8 | v>>8 }

func packetFrame(size int, ipv6, udp, upload bool) []byte {
	frame := make([]byte, size)
	proto, ipSize := byte(unix.IPPROTO_TCP), 20
	if udp {
		proto = unix.IPPROTO_UDP
	}
	if ipv6 {
		binary.BigEndian.PutUint16(frame[12:14], unix.ETH_P_IPV6)
		frame[14], frame[20], frame[21] = 0x60, proto, 64
		binary.BigEndian.PutUint16(frame[18:20], uint16(size-14-40))
		frame[22], frame[38] = 0x20, 0x20
		frame[37], frame[53] = 1, 2
		ipSize = 40
	} else {
		binary.BigEndian.PutUint16(frame[12:14], unix.ETH_P_IP)
		frame[14], frame[22], frame[23] = 0x45, 64, proto
		binary.BigEndian.PutUint16(frame[16:18], uint16(size-14))
		copy(frame[26:30], []byte{198, 18, 0, 1})
		copy(frame[30:34], []byte{198, 18, 0, 2})
	}
	start := 14 + ipSize
	source, dest := uint16(443), uint16(50000)
	if upload {
		source, dest = dest, source
	}
	binary.BigEndian.PutUint16(frame[start:start+2], source)
	binary.BigEndian.PutUint16(frame[start+2:start+4], dest)
	if udp {
		binary.BigEndian.PutUint16(frame[start+4:start+6], uint16(size-start))
	} else {
		frame[start+12] = 0x50
	}
	return frame
}
