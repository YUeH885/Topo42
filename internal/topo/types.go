package topo

import "time"

const Version = "0.7.0"

const AgentOfflineAfter = 90 * time.Second

type InterfaceRead struct {
	Name              string   `json:"name"`
	LatencyMS         *float64 `json:"latency_ms"`
	PacketLossPercent *float64 `json:"packet_loss_percent"`
}

type NodeRead struct {
	Name         string          `json:"name"`
	NodeIPs      []string        `json:"node_ips"`
	AgentVersion string          `json:"agent_version"`
	Online       bool            `json:"online"`
	LastSeenAt   *time.Time      `json:"last_seen_at"`
	Interfaces   []InterfaceRead `json:"interfaces"`
}

type TopologyEdge struct {
	LocalNodeName     string   `json:"local_node_name"`
	PeerNodeName      string   `json:"peer_node_name"`
	Connected         bool     `json:"connected"`
	LatencyMS         *float64 `json:"latency_ms"`
	PacketLossPercent *float64 `json:"packet_loss_percent"`
}

type TopologyRead struct {
	Nodes []NodeRead     `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

type AgentSnapshot struct {
	AgentVersion string          `json:"agent_version"`
	NodeIPs      []string        `json:"node_ips"`
	Interfaces   []InterfaceRead `json:"interfaces"`
}
