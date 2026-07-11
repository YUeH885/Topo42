package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"

	"topo42/internal/topo"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
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
	interfaces := topo.NewInterfaceCache()
	go func() {
		if err := interfaces.WatchNetlink(); err != nil {
			log.Printf("netlink listener stopped: %v", err)
		}
	}()
	for {
		if err := runWebSocketClient(serverURL, nodeName, token, interfaces); err != nil {
			log.Printf("controller websocket failed, retrying: %v", err)
			time.Sleep(topo.DetectionInterval)
		}
	}
}

func runWebSocketClient(serverURL, nodeName, token string, interfaces *topo.InterfaceCache) error {
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	wsURL, err := url.JoinPath(serverURL, "api/agent/ws")
	if err != nil {
		return err
	}
	conn, _, err := (&websocket.Dialer{Proxy: nil}).Dial(wsURL+"?node="+url.QueryEscape(nodeName), header)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("connected to controller node=%s", nodeName)
	peers := map[string][]string{}
	peerUpdates := make(chan map[string][]string, 1)
	readErr := make(chan error, 1)
	go readPeerUpdates(conn, peerUpdates, readErr)
	timer := time.NewTimer(0)
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
		nodeIPs := interfaces.CollectDN42DummyIPs()
		snapshot := topo.AgentSnapshot{
			AgentVersion: topo.Version,
			NodeIPs:      nodeIPs,
			Interfaces:   interfaces.CollectDN42WireGuardDetection(peers, nodeIPs),
		}
		if err := conn.WriteJSON(map[string]any{"event": "snapshot", "payload": snapshot}); err != nil {
			return err
		}
		timer.Reset(topo.DetectionInterval)
	}
}

func readPeerUpdates(conn *websocket.Conn, peerUpdates chan<- map[string][]string, readErr chan<- error) {
	for {
		var event struct {
			Event string              `json:"event"`
			Peers map[string][]string `json:"peers"`
		}
		if err := conn.ReadJSON(&event); err != nil {
			readErr <- err
			return
		}
		if event.Event != "peers" {
			continue
		}
		select {
		case peerUpdates <- event.Peers:
		default:
		}
	}
}
