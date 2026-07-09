package topo

import "time"

const Version = "0.6.3"

const AgentOfflineAfter = 90 * time.Second

type InterfaceRead struct {
	Name              string   `json:"name"`
	PeerNodeIPs       []string `json:"peer_node_ips"`
	RuntimeStatus     string   `json:"runtime_status"`
	LatencyMS         *float64 `json:"latency_ms"`
	PacketLossPercent *float64 `json:"packet_loss_percent"`
}

type NodeRead struct {
	Name         string          `json:"name"`
	NodeIPs      []string        `json:"node_ips"`
	AgentVersion *string         `json:"agent_version"`
	Status       string          `json:"status"`
	LastSeenAt   *time.Time      `json:"last_seen_at"`
	Interfaces   []InterfaceRead `json:"interfaces"`
}

type TopologyEdge struct {
	ID                     string   `json:"id"`
	LocalNodeName          string   `json:"local_node_name"`
	PeerNodeName           string   `json:"peer_node_name"`
	LocalInterfaceName     string   `json:"local_interface_name"`
	PeerInterfaceName      string   `json:"peer_interface_name"`
	LocalStatus            string   `json:"local_status"`
	PeerStatus             string   `json:"peer_status"`
	LocalLatencyMS         *float64 `json:"local_latency_ms"`
	PeerLatencyMS          *float64 `json:"peer_latency_ms"`
	LocalPacketLossPercent *float64 `json:"local_packet_loss_percent"`
	PeerPacketLossPercent  *float64 `json:"peer_packet_loss_percent"`
}

type TopologyRead struct {
	Nodes []NodeRead     `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

type AgentDetectedInterface struct {
	Name              string   `json:"name"`
	RuntimeStatus     string   `json:"runtime_status"`
	LatencyMS         *float64 `json:"latency_ms"`
	PacketLossPercent *float64 `json:"packet_loss_percent"`
}

type AgentSnapshot struct {
	AgentVersion string                   `json:"agent_version"`
	NodeIPs      []string                 `json:"node_ips"`
	Interfaces   []AgentDetectedInterface `json:"interfaces"`
}
