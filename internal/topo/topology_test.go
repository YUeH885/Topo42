package topo

import "testing"

func TestTopologyLinksDetectedInterfaces(t *testing.T) {
	store := NewStore()
	store.SetActive("dn42_cn01", true)
	store.SetActive("dn42_us02", true)
	store.RecordAgentSnapshot("dn42_cn01", AgentSnapshot{
		NodeIPs: []string{"172.23.70.1", "fd6a:93d4:3358::1"},
		Interfaces: []InterfaceRead{{
			Name: "dn42_us02",
		}},
	})
	latency := 10.0
	loss := 33.0
	store.RecordAgentSnapshot("dn42_us02", AgentSnapshot{
		NodeIPs: []string{"172.23.70.2", "fd6a:93d4:3358::2"},
		Interfaces: []InterfaceRead{{
			Name:              "dn42_cn01",
			LatencyMS:         &latency,
			PacketLossPercent: &loss,
		}},
	})

	topology := store.Topology()

	if len(topology.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(topology.Edges))
	}
	edge := topology.Edges[0]
	if !edge.Connected {
		t.Fatal("edge is not connected")
	}
	if edge.LatencyMS == nil || *edge.LatencyMS != 10.0 {
		t.Fatalf("latency = %#v", edge.LatencyMS)
	}
	loss = 100
	if store.Topology().Edges[0].Connected {
		t.Fatal("edge with 100% packet loss is connected")
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
