package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"topo42/internal/topo"
)

func main() {
	if slices.Contains(os.Args[1:], "--version") || slices.Contains(os.Args[1:], "version") {
		println(topo.Version)
		return
	}
	if len(os.Args) != 3 && len(os.Args) != 4 {
		log.Fatal("usage: topo42-agent <server_url> <node_name> [agent_token]")
	}
	serverURL, nodeName := os.Args[1], os.Args[2]
	token := ""
	if len(os.Args) == 4 {
		token = os.Args[3]
	}
	if !topo.NodePattern.MatchString(nodeName) {
		log.Fatal("node_name must match dn42_xxxx, for example dn42_us02")
	}
	for {
		if err := runWebSocketClient(serverURL, nodeName, token); err != nil {
			log.Printf("controller websocket failed, retrying: %v", err)
			time.Sleep(topo.DetectionInterval)
		}
	}
}

func runWebSocketClient(serverURL, nodeName, token string) error {
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	conn, _, err := (&websocket.Dialer{Proxy: nil}).Dial(controllerWSURL(serverURL, nodeName), header)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("connected to controller node=%s", nodeName)
	peers := map[string][]string{}
	peerUpdates := make(chan map[string][]string, 1)
	readErr := make(chan error, 1)
	go readPeerUpdates(conn, peerUpdates, readErr)
	if err := conn.WriteJSON(map[string]any{"event": "hello", "payload": topo.AgentSnapshot{AgentVersion: topo.Version, NodeIPs: topo.CollectDN42DummyIPs()}}); err != nil {
		return err
	}
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case err := <-readErr:
			return err
		case next := <-peerUpdates:
			peers = next
			continue
		case <-timer.C:
		}
		nodeIPs := topo.CollectDN42DummyIPs()
		snapshot := topo.AgentSnapshot{
			AgentVersion: topo.Version,
			NodeIPs:      nodeIPs,
			Interfaces:   topo.CollectDN42WireGuardDetection(peers, nodeIPs),
		}
		if err := conn.WriteJSON(map[string]any{"event": "snapshot", "payload": snapshot}); err != nil {
			return err
		}
		timer.Reset(topo.DetectionInterval)
	}
}

func controllerWSURL(serverURL, nodeName string) string {
	base := strings.TrimRight(serverURL, "/")
	switch {
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + strings.TrimPrefix(base, "http://")
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + strings.TrimPrefix(base, "https://")
	case !strings.HasPrefix(base, "ws://") && !strings.HasPrefix(base, "wss://"):
		base = "ws://" + base
	}
	return base + "/api/agent/ws?node=" + url.QueryEscape(nodeName)
}

func readPeerUpdates(conn *websocket.Conn, peerUpdates chan<- map[string][]string, readErr chan<- error) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			readErr <- err
			return
		}
		next, ok := topo.PeerNodeIPsFromEvent(message)
		if ok {
			select {
			case peerUpdates <- next:
			default:
			}
		}
	}
}
