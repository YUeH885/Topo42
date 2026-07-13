package topo

import (
	"net/netip"
	"regexp"
	"sort"
	"sync"
	"time"
)

var NodePattern = regexp.MustCompile(`^dn42_[a-z]{2}\d{2}$`)

type runtimeNode struct {
	Name         string
	NodeIPs      []string
	AgentVersion string
	LastSeenAt   *time.Time
	Interfaces   map[string]InterfaceRead
	Active       bool
}

type Store struct {
	mu    sync.Mutex
	nodes map[string]*runtimeNode
}

func NewStore() *Store {
	return &Store{nodes: map[string]*runtimeNode{}}
}

func (s *Store) SetActive(name string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureNodeLocked(name).Active = active
}

func (s *Store) RecordAgentSnapshot(name string, snapshot AgentSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node := s.ensureNodeLocked(name)
	node.AgentVersion = snapshot.AgentVersion
	node.NodeIPs = dedupe(snapshot.NodeIPs)
	now := time.Now().UTC()
	node.LastSeenAt = &now
	node.Interfaces = map[string]InterfaceRead{}
	for _, item := range snapshot.Interfaces {
		node.Interfaces[item.Name] = item
	}
}

func (s *Store) PeerNodeIPsFor(name string) map[string][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	peers := map[string][]string{}
	for nodeName, node := range s.nodes {
		if nodeName == name || len(node.NodeIPs) == 0 {
			continue
		}
		peers[nodeName] = append([]string(nil), node.NodeIPs...)
	}
	return peers
}

func (s *Store) Topology() TopologyRead {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	nodes := make([]*runtimeNode, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodeLess(nodes[i], nodes[j]) })

	edges := []TopologyEdge{}
	seenPairs := map[[2]string]bool{}
	readNodes := make([]NodeRead, 0, len(nodes))
	for _, node := range nodes {
		names := make([]string, 0, len(node.Interfaces))
		for name := range node.Interfaces {
			names = append(names, name)
		}
		sort.Strings(names)
		ifaces := make([]InterfaceRead, 0, len(node.Interfaces))
		for _, name := range names {
			iface := node.Interfaces[name]
			ifaces = append(ifaces, iface)
			peer := s.nodes[iface.Name]
			if peer == nil || peer.Name == node.Name {
				continue
			}
			pair := [2]string{min(node.Name, peer.Name), max(node.Name, peer.Name)}
			if seenPairs[pair] {
				continue
			}
			seenPairs[pair] = true
			peerIface, ok := peer.Interfaces[node.Name]
			latency, loss := iface.LatencyMS, iface.PacketLossPercent
			if latency == nil {
				latency = peerIface.LatencyMS
			}
			if loss == nil {
				loss = peerIface.PacketLossPercent
			}
			edges = append(edges, TopologyEdge{
				LocalNodeName:     node.Name,
				PeerNodeName:      peer.Name,
				Connected:         ok && (loss == nil || *loss != 100),
				LatencyMS:         latency,
				PacketLossPercent: loss,
			})
		}
		readNodes = append(readNodes, NodeRead{
			Name:         node.Name,
			NodeIPs:      append([]string(nil), node.NodeIPs...),
			AgentVersion: node.AgentVersion,
			Online:       node.Active && node.LastSeenAt != nil && now.Sub(*node.LastSeenAt) <= AgentOfflineAfter,
			LastSeenAt:   node.LastSeenAt,
			Interfaces:   ifaces,
		})
	}
	return TopologyRead{Nodes: readNodes, Edges: edges}
}

func (s *Store) ensureNodeLocked(name string) *runtimeNode {
	node := s.nodes[name]
	if node == nil {
		node = &runtimeNode{Name: name, NodeIPs: []string{}, Interfaces: map[string]InterfaceRead{}}
		s.nodes[name] = node
	}
	return node
}

func nodeLess(a, b *runtimeNode) bool {
	aAddr, aOK := firstNodeAddr(a.NodeIPs)
	bAddr, bOK := firstNodeAddr(b.NodeIPs)
	if aOK != bOK {
		return aOK
	}
	if aOK && bOK {
		if aAddr.Is4() != bAddr.Is4() {
			return aAddr.Is4()
		}
		if cmp := aAddr.Compare(bAddr); cmp != 0 {
			return cmp < 0
		}
	}
	return a.Name < b.Name
}

func firstNodeAddr(values []string) (netip.Addr, bool) {
	for _, value := range values {
		addr, err := netip.ParseAddr(value)
		if err == nil {
			return addr, true
		}
	}
	return netip.Addr{}, false
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result
}
