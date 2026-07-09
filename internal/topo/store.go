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
	LastSeenAt   time.Time
	Interfaces   map[string]InterfaceRead
}

type Store struct {
	mu     sync.Mutex
	nodes  map[string]*runtimeNode
	active map[string]bool
}

func NewStore() *Store {
	return &Store{
		nodes:  map[string]*runtimeNode{},
		active: map[string]bool{},
	}
}

func (s *Store) SetActive(name string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureNodeLocked(name)
	if active {
		s.active[name] = true
		return
	}
	delete(s.active, name)
}

func (s *Store) RecordAgentHello(name string, snapshot AgentSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node := s.ensureNodeLocked(name)
	if snapshot.AgentVersion != "" {
		node.AgentVersion = snapshot.AgentVersion
	}
	node.NodeIPs = dedupe(snapshot.NodeIPs)
	node.LastSeenAt = time.Now().UTC()
}

func (s *Store) RecordAgentSnapshot(name string, snapshot AgentSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node := s.ensureNodeLocked(name)
	if snapshot.AgentVersion != "" {
		node.AgentVersion = snapshot.AgentVersion
	}
	node.NodeIPs = dedupe(snapshot.NodeIPs)
	node.LastSeenAt = time.Now().UTC()
	node.Interfaces = map[string]InterfaceRead{}
	for _, item := range snapshot.Interfaces {
		status := item.RuntimeStatus
		if status == "" {
			status = "unknown"
		}
		node.Interfaces[item.Name] = InterfaceRead{
			Name:              item.Name,
			RuntimeStatus:     status,
			LatencyMS:         item.LatencyMS,
			PacketLossPercent: item.PacketLossPercent,
		}
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
	for _, local := range nodes {
		names := make([]string, 0, len(local.Interfaces))
		for name := range local.Interfaces {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			iface := local.Interfaces[name]
			peer := s.nodes[iface.Name]
			if peer == nil || peer.Name == local.Name {
				continue
			}
			pair := sortedPair(local.Name, peer.Name)
			if seenPairs[pair] {
				continue
			}
			seenPairs[pair] = true
			peerIface, ok := peer.Interfaces[local.Name]
			peerName := local.Name
			peerStatus := "unknown"
			var peerLatency, peerLoss *float64
			if ok {
				peerName = peerIface.Name
				peerStatus = peerIface.RuntimeStatus
				peerLatency = peerIface.LatencyMS
				peerLoss = peerIface.PacketLossPercent
			}
			edges = append(edges, TopologyEdge{
				ID:                     "wg-" + pair[0] + "-" + pair[1],
				LocalNodeName:          local.Name,
				PeerNodeName:           peer.Name,
				LocalInterfaceName:     iface.Name,
				PeerInterfaceName:      peerName,
				LocalStatus:            iface.RuntimeStatus,
				PeerStatus:             peerStatus,
				LocalLatencyMS:         iface.LatencyMS,
				PeerLatencyMS:          peerLatency,
				LocalPacketLossPercent: iface.PacketLossPercent,
				PeerPacketLossPercent:  peerLoss,
			})
		}
	}

	readNodes := make([]NodeRead, 0, len(nodes))
	for _, node := range nodes {
		ifaces := make([]InterfaceRead, 0, len(node.Interfaces))
		names := make([]string, 0, len(node.Interfaces))
		for name := range node.Interfaces {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			iface := node.Interfaces[name]
			if peer := s.nodes[iface.Name]; peer != nil {
				iface.PeerNodeIPs = append([]string(nil), peer.NodeIPs...)
			} else {
				iface.PeerNodeIPs = []string{}
			}
			ifaces = append(ifaces, iface)
		}
		var version *string
		if node.AgentVersion != "" {
			value := node.AgentVersion
			version = &value
		}
		var lastSeen *time.Time
		if !node.LastSeenAt.IsZero() {
			value := node.LastSeenAt
			lastSeen = &value
		}
		readNodes = append(readNodes, NodeRead{
			Name:         node.Name,
			NodeIPs:      append([]string(nil), node.NodeIPs...),
			AgentVersion: version,
			Status:       s.nodeStatusLocked(node, now),
			LastSeenAt:   lastSeen,
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

func (s *Store) nodeStatusLocked(node *runtimeNode, now time.Time) string {
	if !s.active[node.Name] || node.LastSeenAt.IsZero() {
		return "offline"
	}
	if now.Sub(node.LastSeenAt) <= AgentOfflineAfter {
		return "online"
	}
	return "offline"
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

func sortedPair(a, b string) [2]string {
	if a < b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
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
