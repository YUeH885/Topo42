package topo

import "testing"

func TestTopologyLinksDetectedInterfaces(t *testing.T) {
	store := NewStore()
	store.SetActive("dn42_cn01", true)
	store.SetActive("dn42_us02", true)
	store.RecordAgentSnapshot("dn42_cn01", AgentSnapshot{
		NodeIPs: []string{"172.23.70.1", "fd6a:93d4:3358::1"},
		Interfaces: []AgentDetectedInterface{{
			Name:          "dn42_us02",
			RuntimeStatus: "running",
		}},
	})
	latency := 10.0
	loss := 33.0
	store.RecordAgentSnapshot("dn42_us02", AgentSnapshot{
		NodeIPs: []string{"172.23.70.2", "fd6a:93d4:3358::2"},
		Interfaces: []AgentDetectedInterface{{
			Name:              "dn42_cn01",
			RuntimeStatus:     "running",
			LatencyMS:         &latency,
			PacketLossPercent: &loss,
		}},
	})

	topology := store.Topology()

	if len(topology.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(topology.Edges))
	}
	edge := topology.Edges[0]
	if edge.LocalStatus != "running" || edge.PeerStatus != "running" {
		t.Fatalf("edge statuses = %s/%s", edge.LocalStatus, edge.PeerStatus)
	}
	if edge.LocalLatencyMS != nil || edge.PeerLatencyMS == nil || *edge.PeerLatencyMS != 10.0 {
		t.Fatalf("latency = %#v/%#v", edge.LocalLatencyMS, edge.PeerLatencyMS)
	}
	nodes := map[string]NodeRead{}
	for _, node := range topology.Nodes {
		nodes[node.Name] = node
	}
	if got := nodes["dn42_cn01"].Interfaces[0].PeerNodeIPs[0]; got != "172.23.70.2" {
		t.Fatalf("peer ip = %s", got)
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
