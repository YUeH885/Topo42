package topo

import (
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

const DetectionInterval = 30 * time.Second
const dn42DummyInterface = "dn42_dummy"
const pingCount = 10
const pingTimeout = time.Second

var runPing = icmpPingStats

func CollectDN42Detection(peers map[string][]string) ([]string, []InterfaceRead) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return []string{}, []InterfaceRead{}
	}
	nodeIPs := []string{}
	names := []string{}
	for _, item := range interfaces {
		if item.Name == dn42DummyInterface {
			addresses, _ := item.Addrs()
			for _, address := range addresses {
				if ip := nodeIP(address.String()); ip != "" {
					nodeIPs = append(nodeIPs, ip)
				}
			}
		}
		if NodePattern.MatchString(item.Name) {
			names = append(names, item.Name)
		}
	}
	detected := []InterfaceRead{}
	for _, name := range names {
		latency, loss := pingStats(name, peers[name], nodeIPs)
		detected = append(detected, InterfaceRead{
			Name:              name,
			LatencyMS:         latency,
			PacketLossPercent: loss,
		})
	}
	return nodeIPs, detected
}

func nodeIP(value string) string {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return ""
	}
	addr := prefix.Addr()
	text := addr.String()
	if addr.IsLinkLocalUnicast() || strings.HasSuffix(text, ".42") || strings.HasSuffix(text, ":42") {
		return ""
	}
	return text
}

func dummyIPv6ToLinkLocal(value string) string {
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.Is6() || addr.IsLinkLocalUnicast() {
		return ""
	}
	bytes := addr.As16()
	parts := []string{}
	for index := 2; index < len(bytes); index += 2 {
		group := binary.BigEndian.Uint16(bytes[index : index+2])
		if group != 0 {
			parts = append(parts, strconv.FormatUint(uint64(group), 16))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "fe80::" + strings.Join(parts, ":")
}

func peerIPIsSmaller(localNodeIPs, peerNodeIPs []string) bool {
	for _, want4 := range []bool{true, false} {
		localIP, localOK := firstAddrVersion(localNodeIPs, want4)
		peerIP, peerOK := firstAddrVersion(peerNodeIPs, want4)
		if localOK && peerOK {
			return peerIP.Compare(localIP) < 0
		}
	}
	return false
}

func pingStats(interfaceName string, peerNodeIPs, localNodeIPs []string) (*float64, *float64) {
	if !peerIPIsSmaller(localNodeIPs, peerNodeIPs) {
		return nil, nil
	}
	for _, value := range peerNodeIPs {
		address := dummyIPv6ToLinkLocal(value)
		if address == "" {
			continue
		}
		latency, loss := runPing(interfaceName, address)
		if latency != nil || loss != nil {
			return latency, loss
		}
	}
	return nil, nil
}

func icmpPingStats(interfaceName, address string) (*float64, *float64) {
	ip := net.ParseIP(address)
	if ip == nil {
		return nil, nil
	}
	conn, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		return nil, nil
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff
	target := &net.IPAddr{IP: ip, Zone: interfaceName}
	buffer := make([]byte, 1500)
	received := 0
	total := time.Duration(0)
	for seq := 0; seq < pingCount; seq++ {
		message := icmp.Message{
			Type: ipv6.ICMPTypeEchoRequest,
			Body: &icmp.Echo{ID: id, Seq: seq, Data: []byte("topo42")},
		}
		raw, err := message.Marshal(nil)
		if err != nil {
			continue
		}
		start := time.Now()
		if _, err := conn.WriteTo(raw, target); err != nil {
			continue
		}
		if readICMPEchoReply(conn, buffer, id, seq, start.Add(pingTimeout)) {
			received++
			total += time.Since(start)
		}
	}

	loss := float64(pingCount-received) * 100 / pingCount
	if received == 0 {
		return nil, &loss
	}
	latency := float64(total.Microseconds()) / 1000 / float64(received)
	return &latency, &loss
}

func readICMPEchoReply(conn *icmp.PacketConn, buffer []byte, id, seq int, deadline time.Time) bool {
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return false
		}
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			return false
		}
		message, err := icmp.ParseMessage(ipv6.ICMPTypeEchoReply.Protocol(), buffer[:n])
		if err != nil || message.Type != ipv6.ICMPTypeEchoReply {
			continue
		}
		echo, ok := message.Body.(*icmp.Echo)
		if ok && echo.ID == id && echo.Seq == seq {
			return true
		}
	}
}

func firstAddrVersion(values []string, want4 bool) (netip.Addr, bool) {
	for _, value := range values {
		addr, err := netip.ParseAddr(value)
		if err == nil && addr.Is4() == want4 {
			return addr, true
		}
	}
	return netip.Addr{}, false
}
