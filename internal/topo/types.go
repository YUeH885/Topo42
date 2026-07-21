package topo

import "time"

const Version = "0.8.0"

const AgentOfflineAfter = 90 * time.Second

type InterfaceRead struct {
	Name              string   `json:"name"`
	LocalAddress      string   `json:"local_address,omitempty"`
	PeerAddress       string   `json:"peer_address,omitempty"`
	PeerNodeName      string   `json:"peer_node_name,omitempty"`
	BabelNeighbor     bool     `json:"babel_neighbor,omitempty"`
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
	LocalNodeName string `json:"local_node_name"`
	PeerNodeName  string `json:"peer_node_name"`
	Connected     bool   `json:"connected"`
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
