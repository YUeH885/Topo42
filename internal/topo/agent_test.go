package topo

import (
	"bufio"
	"fmt"
	"net"
	"testing"
)

func TestBabelDetection(t *testing.T) {
	socketPath := t.TempDir() + "/babeld.sock"
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		_, _ = fmt.Fprint(conn, "BABEL 1.0\nversion babeld-1.14\nhost test\nmy-id 00:00:00:00:00:00:00:01\nok\n")
		scanner := bufio.NewScanner(conn)
		if !scanner.Scan() || scanner.Text() != "dump" {
			serverErr <- fmt.Errorf("request = %q", scanner.Text())
			return
		}
		_, _ = fmt.Fprint(conn, "add interface wg-paris up true ipv6 fe80::1\n")
		_, _ = fmt.Fprint(conn, "add interface transit.foo up false\n")
		_, _ = fmt.Fprint(conn, "add interface eth0 up true ipv6 fe80::3\n")
		_, _ = fmt.Fprint(conn, "add interface wg-dead up true ipv6 fe80::5\n")
		_, _ = fmt.Fprint(conn, "add neighbour 1 address fe80::2 if wg-paris reach efff ureach 0000 rxcost 96 txcost 96 rtt 12.345 rttcost 2 cost 98\n")
		_, _ = fmt.Fprint(conn, "add neighbour 2 address fe80::4 if eth0 reach ffff ureach 0000 rxcost 96 txcost 96 rtt 1.000 rttcost 0 cost 96\n")
		_, _ = fmt.Fprint(conn, "add neighbour 3 address fe80::6 if wg-dead reach 0000 ureach 0000 rxcost 65535 txcost 65535 cost 65535\n")
		_, _ = fmt.Fprint(conn, "ok\n")
		serverErr <- nil
	}()

	interfaces, err := collectBabelInterfaces(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	if len(interfaces) != 4 {
		t.Fatalf("interfaces = %#v", interfaces)
	}
	if got := interfaces[0]; got.Name != "eth0" || !got.BabelNeighbor {
		t.Fatalf("first interface = %#v", got)
	}
	if got := interfaces[1]; got.Name != "transit.foo" || got.BabelNeighbor {
		t.Fatalf("second interface = %#v", got)
	}
	if got := interfaces[2]; got.Name != "wg-dead" || got.BabelNeighbor {
		t.Fatalf("dead interface = %#v", got)
	}
	got := interfaces[3]
	if got.Name != "wg-paris" || got.LocalAddress != "fe80::1" || got.PeerAddress != "fe80::2" || !got.BabelNeighbor {
		t.Fatalf("interface = %#v", got)
	}
	if got.LatencyMS == nil || *got.LatencyMS != 12.345 {
		t.Fatalf("latency = %#v", got.LatencyMS)
	}
	if got.PacketLossPercent == nil || *got.PacketLossPercent != 6.25 {
		t.Fatalf("loss = %#v", got.PacketLossPercent)
	}
	if loss := babelHelloLoss("f000", "0000"); loss == nil || *loss != 0 {
		t.Fatalf("startup loss = %#v", loss)
	}
}
