package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"

	"topo42/internal/topo"
)

//go:embed web/*
var embeddedWeb embed.FS

var store = topo.NewStore()
var sessionsMu sync.Mutex
var sessions = map[string]*websocket.Conn{}
var agentToken string

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		println(topo.Version)
		return
	}
	addr := flag.String("addr", "0.0.0.0:8000", "")
	flag.StringVar(&agentToken, "agent-token", "", "")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/topology", topologyHandler)
	mux.HandleFunc("GET /api/agent/ws", agentWSHandler)
	web, _ := fs.Sub(embeddedWeb, "web")
	mux.Handle("GET /", http.FileServer(http.FS(web)))

	log.Printf("Topo42 controller starting version=%s addr=%s", topo.Version, *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func topologyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(store.Topology())
}

func agentWSHandler(w http.ResponseWriter, r *http.Request) {
	node := r.URL.Query().Get("node")
	if node == "" {
		http.Error(w, "invalid node", http.StatusForbidden)
		return
	}
	if agentToken != "" && r.Header.Get("Authorization") != "Bearer "+agentToken {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}
	conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
	if err != nil {
		return
	}
	sessionsMu.Lock()
	old := sessions[node]
	sessions[node] = conn
	sessionsMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	store.SetActive(node, true)
	defer func() {
		sessionsMu.Lock()
		if sessions[node] == conn {
			delete(sessions, node)
			store.SetActive(node, false)
		}
		sessionsMu.Unlock()
		_ = conn.Close()
	}()
	log.Printf("agent connected node=%s", node)
	for {
		var snapshot topo.AgentSnapshot
		if err := conn.ReadJSON(&snapshot); err != nil {
			log.Printf("agent disconnected node=%s", node)
			return
		}
		store.RecordAgentSnapshot(node, snapshot)
	}
}
