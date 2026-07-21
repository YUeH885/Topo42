package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"

	"topo42/internal/topo"
)

var collectDetection = topo.CollectDN42Detection

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		println(topo.Version)
		return
	}
	babelAddress := flag.String("babel-address", "[::1]:33123", "")
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 && len(args) != 3 {
		log.Fatal("usage: topo42-agent [--babel-address address] <server_url> <node_name> [agent_token]")
	}
	serverURL, nodeName := args[0], args[1]
	token := ""
	if len(args) == 3 {
		token = args[2]
	}
	for {
		log.Printf("agent cycle failed, retrying: %v", runWebSocketClient(serverURL, nodeName, token, *babelAddress, topo.DetectionInterval))
		time.Sleep(topo.DetectionInterval)
	}
}

func runWebSocketClient(serverURL, nodeName, token, babelAddress string, interval time.Duration) error {
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
		nodeIPs, interfaces, err := collectDetection(babelAddress)
		if err != nil {
			return err
		}
		if err := conn.WriteJSON(topo.AgentSnapshot{
			AgentVersion: topo.Version,
			NodeIPs:      nodeIPs,
			Interfaces:   interfaces,
		}); err != nil {
			return err
		}
		// ponytail: disconnect detection can lag one interval; restore concurrent reads only if faster recovery matters.
		time.Sleep(interval)
	}
}
