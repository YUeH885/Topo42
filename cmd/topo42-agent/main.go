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
	for {
		if err := runWebSocketClient(serverURL, nodeName, token, topo.DetectionInterval); err != nil {
			log.Printf("controller websocket failed, retrying: %v", err)
			time.Sleep(topo.DetectionInterval)
		}
	}
}

func runWebSocketClient(serverURL, nodeName, token string, interval time.Duration) error {
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
	for {
		var peers map[string][]string
		if err := conn.ReadJSON(&peers); err != nil {
			return err
		}
		nodeIPs, interfaces := topo.CollectDN42Detection(peers)
		snapshot := topo.AgentSnapshot{
			AgentVersion: topo.Version,
			NodeIPs:      nodeIPs,
			Interfaces:   interfaces,
		}
		if err := conn.WriteJSON(snapshot); err != nil {
			return err
		}
		// ponytail: disconnect detection can lag one interval; restore concurrent reads only if faster recovery matters.
		time.Sleep(interval)
	}
}
