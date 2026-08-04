package topo

import (
	"slices"
	"testing"
)

func TestTopologyLinksDetectedInterfaces(t *testing.T) {
	store := NewStore()
	store.SetActive("dn42_cn01", true)
	store.SetActive("dn42_us02", true)
	localLatency := 20.0
	localLoss := 100.0
	store.RecordAgentSnapshot("dn42_cn01", AgentSnapshot{
		NodeIPs: []string{"172.23.70.1", "172.23.70.42", "fd6a:93d4:3358::1", "fe80::1"},
		Interfaces: []InterfaceRead{{
			Name:              "wg-us",
			LocalAddress:      "fe80::1",
			PeerAddress:       "fe80::2",
			BabelNeighbor:     true,
			LatencyMS:         &localLatency,
			PacketLossPercent: &localLoss,
		}},
	})
	peerLatency := 10.0
	peerLoss := 33.0
	store.RecordAgentSnapshot("dn42_us02", AgentSnapshot{
		NodeIPs: []string{"172.23.70.2", "172.23.70.42", "fd6a:93d4:3358::2"},
		Interfaces: []InterfaceRead{{
			Name:              "tunnel.cn",
			LocalAddress:      "fe80::2",
			PeerAddress:       "fe80::1",
			BabelNeighbor:     true,
			LatencyMS:         &peerLatency,
			PacketLossPercent: &peerLoss,
		}},
	})

	topology := store.Topology()

	if len(topology.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(topology.Edges))
	}
	if !topology.Edges[0].Connected {
		t.Fatal("edge is not connected")
	}
	if !slices.Equal(topology.Nodes[0].NodeIPs, []string{"172.23.70.1", "fd6a:93d4:3358::1"}) {
		t.Fatalf("node IPs = %#v", topology.Nodes[0].NodeIPs)
	}
	if !slices.Equal(topology.Nodes[1].NodeIPs, []string{"172.23.70.2", "fd6a:93d4:3358::2"}) {
		t.Fatalf("peer node IPs = %#v", topology.Nodes[1].NodeIPs)
	}
	local := topology.Nodes[0].Interfaces[0]
	peer := topology.Nodes[1].Interfaces[0]
	if local.LatencyMS == nil || *local.LatencyMS != 20.0 || peer.LatencyMS == nil || *peer.LatencyMS != 10.0 {
		t.Fatalf("latencies = %#v / %#v", local.LatencyMS, peer.LatencyMS)
	}
	if local.PacketLossPercent == nil || *local.PacketLossPercent != 100.0 || peer.PacketLossPercent == nil || *peer.PacketLossPercent != 33.0 {
		t.Fatalf("losses = %#v / %#v", local.PacketLossPercent, peer.PacketLossPercent)
	}
	if got := local.PeerNodeName; got != "dn42_us02" {
		t.Fatalf("peer node = %q", got)
	}
	store.RecordAgentSnapshot("dn42_cn01", AgentSnapshot{
		NodeIPs:    []string{"172.23.70.1", "fd6a:93d4:3358::1"},
		Interfaces: []InterfaceRead{{Name: "wg-us", LocalAddress: "fe80::1"}},
	})
	topology = store.Topology()
	if len(topology.Edges) != 1 || topology.Edges[0].Connected {
		t.Fatalf("missing Babel neighbour edge = %#v", topology.Edges)
	}
	store.RecordAgentSnapshot("dn42_us02", AgentSnapshot{
		NodeIPs: []string{"172.23.70.2", "fd6a:93d4:3358::2"},
		Interfaces: []InterfaceRead{{
			Name:         "tunnel.cn",
			LocalAddress: "fe80::2",
		}},
	})
	topology = store.Topology()
	if len(topology.Edges) != 1 || topology.Edges[0].Connected {
		t.Fatalf("stale Babel neighbour edge = %#v", topology.Edges)
	}
	for _, node := range topology.Nodes {
		if len(node.Interfaces) != 1 || node.Interfaces[0].PeerNodeName == "" {
			t.Fatalf("stale peer mapping for %s = %#v", node.Name, node.Interfaces)
		}
	}
}

func TestTopologySortsNodesByIP(t *testing.T) {
	store := NewStore()
	store.RecordAgentSnapshot("dn42_us10", AgentSnapshot{NodeIPs: []string{"172.23.70.10"}})
	store.RecordAgentSnapshot("dn42_cn02", AgentSnapshot{NodeIPs: []string{"172.23.70.2"}})
	store.RecordAgentSnapshot("dn42_zz99", AgentSnapshot{})

	topology := store.Topology()

	want := []string{"dn42_cn02", "dn42_us10", "dn42_zz99"}
	for index, name := range want {
		if topology.Nodes[index].Name != name {
			t.Fatalf("node[%d] = %s, want %s", index, topology.Nodes[index].Name, name)
		}
	}
}
