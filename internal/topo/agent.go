package topo

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DetectionInterval = 30 * time.Second
const dn42DummyInterface = "dn42_dummy"

var pingAvgPattern = regexp.MustCompile(`=\s*[0-9.]+/([0-9.]+)/`)
var pingLossPattern = regexp.MustCompile(`([0-9.]+)%\s*packet loss`)

type commandResult struct {
	stdout     string
	returnCode int
}

var runCommand = func(command []string, timeout time.Duration) commandResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return commandResult{returnCode: 124}
	}
	if err == nil {
		return commandResult{stdout: string(output), returnCode: 0}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return commandResult{stdout: string(output), returnCode: exitErr.ExitCode()}
	}
	return commandResult{stdout: string(output), returnCode: 1}
}

func PeerNodeIPsFromEvent(raw []byte) (map[string][]string, bool) {
	var event struct {
		Event string              `json:"event"`
		Peers map[string][]string `json:"peers"`
	}
	if err := json.Unmarshal(raw, &event); err != nil || event.Event != "peers" {
		return nil, false
	}
	return event.Peers, true
}

func CollectDN42DummyIPs() []string {
	result := []string{}
	for _, value := range interfaceAddresses(dn42DummyInterface) {
		if ip := nodeIP(value); ip != "" {
			result = append(result, ip)
		}
	}
	return dedupe(result)
}

func CollectDN42WireGuardDetection(peers map[string][]string, localNodeIPs []string) []AgentDetectedInterface {
	detected := []AgentDetectedInterface{}
	for _, name := range wireguardInterfaces() {
		latency, loss := pingStats(name, peers[name], localNodeIPs)
		detected = append(detected, AgentDetectedInterface{
			Name:              name,
			RuntimeStatus:     "running",
			LatencyMS:         latency,
			PacketLossPercent: loss,
		})
	}
	return detected
}

func wireguardInterfaces() []string {
	result := runCommand([]string{"wg", "show", "interfaces"}, 30*time.Second)
	if result.returnCode != 0 {
		return []string{}
	}
	names := []string{}
	for _, name := range strings.Fields(result.stdout) {
		if NodePattern.MatchString(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func interfaceAddresses(name string) []string {
	result := runCommand([]string{"ip", "-o", "addr", "show", "dev", name}, 30*time.Second)
	if result.returnCode != 0 {
		return []string{}
	}
	addresses := []string{}
	for _, line := range strings.Split(result.stdout, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 4 && (parts[2] == "inet" || parts[2] == "inet6") {
			addresses = append(addresses, parts[3])
		}
	}
	return addresses
}

func nodeIP(value string) string {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		addr, parseErr := netip.ParseAddr(value)
		if parseErr != nil {
			return ""
		}
		return filterNodeAddr(addr)
	}
	return filterNodeAddr(prefix.Addr())
}

func filterNodeAddr(addr netip.Addr) string {
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
	local := parseAddrs(localNodeIPs)
	peer := parseAddrs(peerNodeIPs)
	for _, want4 := range []bool{true, false} {
		localIP, localOK := firstAddrVersion(local, want4)
		peerIP, peerOK := firstAddrVersion(peer, want4)
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
		result := runCommand([]string{"ping", "-c", "10", "-W", "1", "-I", interfaceName, address}, 12*time.Second)
		var latency, loss *float64
		if match := pingAvgPattern.FindStringSubmatch(result.stdout); len(match) == 2 {
			if value, err := strconv.ParseFloat(match[1], 64); err == nil {
				latency = &value
			}
		}
		if match := pingLossPattern.FindStringSubmatch(result.stdout); len(match) == 2 {
			if value, err := strconv.ParseFloat(match[1], 64); err == nil {
				loss = &value
			}
		}
		if latency != nil || loss != nil {
			return latency, loss
		}
	}
	return nil, nil
}

func parseAddrs(values []string) []netip.Addr {
	result := []netip.Addr{}
	for _, value := range values {
		addr, err := netip.ParseAddr(value)
		if err == nil {
			result = append(result, addr)
		}
	}
	return result
}

func firstAddrVersion(values []netip.Addr, want4 bool) (netip.Addr, bool) {
	for _, value := range values {
		if value.Is4() == want4 {
			return value, true
		}
	}
	return netip.Addr{}, false
}
