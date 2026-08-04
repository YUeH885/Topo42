package topo

import (
	"net/netip"
	"slices"
	"sort"
	"sync"
	"time"
)

type runtimeNode struct {
	Name         string
	NodeIPs      []string
	AgentVersion string
	LastSeenAt   *time.Time
	Interfaces   []InterfaceRead
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
	node.NodeIPs = snapshot.NodeIPs
	now := time.Now().UTC()
	node.LastSeenAt = &now
	node.Interfaces = retainPeerAddresses(node.Interfaces, snapshot.Interfaces)
}

func (s *Store) Topology() TopologyRead {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	nodes := make([]*runtimeNode, 0, len(s.nodes))
	ipNodeCounts := map[string]int{}
	for _, node := range s.nodes {
		nodes = append(nodes, node)
		for _, ip := range node.NodeIPs {
			ipNodeCounts[ip]++
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodeLess(nodes[i], nodes[j]) })

	edges := []TopologyEdge{}
	// ponytail: 当前 UI 每对节点只画一条边；需要展示并行链路时再把接口地址加入边 ID。
	seenPairs := map[[2]string]bool{}
	readNodes := make([]NodeRead, 0, len(nodes))
	addressOwners := interfaceAddressOwners(nodes)
	for _, node := range nodes {
		nodeIPs := []string{}
		for _, ip := range node.NodeIPs {
			addr, _ := netip.ParseAddr(ip)
			if ipNodeCounts[ip] == 1 && !(addr.Is6() && addr.IsLinkLocalUnicast()) {
				nodeIPs = append(nodeIPs, ip)
			}
		}
		ifaces := append([]InterfaceRead(nil), node.Interfaces...)
		for index, iface := range ifaces {
			peer := addressOwners[iface.PeerAddress]
			if peer == nil || peer == node {
				continue
			}
			ifaces[index].PeerNodeName = peer.Name
			pair := [2]string{min(node.Name, peer.Name), max(node.Name, peer.Name)}
			if seenPairs[pair] {
				continue
			}
			seenPairs[pair] = true
			connected := iface.BabelNeighbor && slices.ContainsFunc(peer.Interfaces, func(peerIface InterfaceRead) bool {
				return peerIface.PeerAddress == iface.LocalAddress && peerIface.BabelNeighbor
			})
			edges = append(edges, TopologyEdge{LocalNodeName: node.Name, PeerNodeName: peer.Name, Connected: connected})
		}
		readNodes = append(readNodes, NodeRead{
			Name:         node.Name,
			NodeIPs:      nodeIPs,
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
		node = &runtimeNode{Name: name, NodeIPs: []string{}, Interfaces: []InterfaceRead{}}
		s.nodes[name] = node
	}
	return node
}

func interfaceAddressOwners(nodes []*runtimeNode) map[string]*runtimeNode {
	owners := map[string]*runtimeNode{}
	for _, node := range nodes {
		for _, iface := range node.Interfaces {
			if iface.LocalAddress != "" {
				owners[iface.LocalAddress] = node
			}
		}
	}
	return owners
}

// ponytail: 仅保留当前进程内的上一帧；需要跨 Controller 重启保持链路时再持久化历史。
func retainPeerAddresses(previous, current []InterfaceRead) []InterfaceRead {
	previousByName := map[string]InterfaceRead{}
	for _, iface := range previous {
		if iface.PeerAddress != "" {
			previousByName[iface.Name] = iface
		}
	}
	result := slices.Clone(current)
	for index := range result {
		previous, ok := previousByName[result[index].Name]
		if ok && result[index].PeerAddress == "" && result[index].LocalAddress == previous.LocalAddress {
			result[index].PeerAddress = previous.PeerAddress
		}
	}
	return result
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
