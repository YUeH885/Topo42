package topo

import (
	"bufio"
	"errors"
	"fmt"
	"math/bits"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DetectionInterval = 30 * time.Second
const babelTimeout = 3 * time.Second

func CollectDN42Detection(babelAddress, dummyInterface string) ([]string, []InterfaceRead, error) {
	nodeIPs := collectNodeIPs(dummyInterface)
	interfaces, err := collectBabelInterfaces(babelAddress)
	return nodeIPs, interfaces, err
}

func collectNodeIPs(interfaceName string) []string {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return []string{}
	}
	addresses, _ := iface.Addrs()
	result := []string{}
	for _, address := range addresses {
		if prefix, err := netip.ParsePrefix(address.String()); err == nil {
			result = append(result, prefix.Addr().String())
		}
	}
	return result
}

func collectBabelInterfaces(address string) ([]InterfaceRead, error) {
	network := "tcp"
	if strings.HasPrefix(address, "/") {
		network = "unix"
	}
	conn, err := net.DialTimeout(network, address, babelTimeout)
	if err != nil {
		return nil, fmt.Errorf("connect to babeld: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(babelTimeout))
	scanner := bufio.NewScanner(conn)
	header, err := readBabelReply(scanner)
	if err != nil {
		return nil, fmt.Errorf("read babeld header: %w", err)
	}
	if len(header) == 0 || header[0] != "BABEL 1.0" {
		return nil, errors.New("unsupported babeld local protocol")
	}
	if _, err := fmt.Fprintln(conn, "dump"); err != nil {
		return nil, fmt.Errorf("request babeld dump: %w", err)
	}
	lines, err := readBabelReply(scanner)
	if err != nil {
		return nil, fmt.Errorf("read babeld dump: %w", err)
	}
	return parseBabelDump(lines), nil
}

func readBabelReply(scanner *bufio.Scanner) ([]string, error) {
	lines := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "ok":
			return lines, nil
		case line == "bad" || line == "no" || strings.HasPrefix(line, "no "):
			return nil, errors.New(line)
		default:
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("babeld closed the connection")
}

func parseBabelDump(lines []string) []InterfaceRead {
	locals := map[string]string{}
	neighbours := map[string][]InterfaceRead{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "add" {
			continue
		}
		switch fields[1] {
		case "interface":
			name := fields[2]
			locals[name] = babelField(fields, "ipv6")
		case "neighbour":
			name := babelField(fields, "if")
			if name == "" {
				continue
			}
			neighbour := InterfaceRead{
				Name:        name,
				PeerAddress: babelField(fields, "address"),
			}
			cost, costErr := strconv.ParseUint(babelField(fields, "cost"), 10, 16)
			neighbour.BabelNeighbor = costErr != nil || cost < 0xffff
			if value, err := strconv.ParseFloat(babelField(fields, "rtt"), 64); err == nil {
				neighbour.LatencyMS = &value
			}
			if loss := babelHelloLoss(babelField(fields, "reach"), babelField(fields, "ureach")); loss != nil {
				neighbour.PacketLossPercent = loss
			}
			neighbours[name] = append(neighbours[name], neighbour)
		}
	}

	result := []InterfaceRead{}
	for name, localAddress := range locals {
		items := neighbours[name]
		if len(items) == 0 {
			items = []InterfaceRead{{Name: name}}
		}
		for _, item := range items {
			item.LocalAddress = localAddress
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].PeerAddress < result[j].PeerAddress
	})
	return result
}

func babelField(fields []string, name string) string {
	for index := 0; index+1 < len(fields); index++ {
		if fields[index] == name {
			return fields[index+1]
		}
	}
	return ""
}

func babelHelloLoss(reachValue, unicastReachValue string) *float64 {
	if reachValue == "0000" {
		reachValue = unicastReachValue
	}
	reach, err := strconv.ParseUint(reachValue, 16, 16)
	if err != nil {
		return nil
	}
	if reach == 0 {
		loss := 100.0
		return &loss
	}
	value := uint16(reach)
	// 只统计首次收到 Hello 后的窗口，避免把启动阶段尚未填充的低位算作丢包。
	samples := 16 - bits.TrailingZeros16(value)
	loss := float64(samples-bits.OnesCount16(value)) * 100 / float64(samples)
	return &loss
}
