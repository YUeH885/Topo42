package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"

	"topo42/internal/topo"
)

//go:embed web/*
var embeddedWeb embed.FS

var store = topo.NewStore()
var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
var sessionsMu sync.Mutex
var sessions = map[string]*websocket.Conn{}
var agentToken string

func main() {
	if slices.Contains(os.Args[1:], "--version") || slices.Contains(os.Args[1:], "version") {
		println(topo.Version)
		return
	}
	host := flag.String("host", "0.0.0.0", "")
	port := flag.Int("port", 8000, "")
	flag.StringVar(&agentToken, "agent-token", "", "")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/topology", topologyHandler)
	mux.HandleFunc("/api/agent/ws", agentWSHandler)
	web, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(web)))

	addr := *host + ":" + strconv.Itoa(*port)
	log.Printf("Topo42 controller starting version=%s addr=%s", topo.Version, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func topologyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(store.Topology()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func agentWSHandler(w http.ResponseWriter, r *http.Request) {
	node := r.URL.Query().Get("node")
	if !topo.NodePattern.MatchString(node) {
		http.Error(w, "invalid node", http.StatusForbidden)
		return
	}
	if agentToken != "" && r.Header.Get("Authorization") != "Bearer "+agentToken {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
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
	if err := conn.WriteJSON(map[string]any{"event": "peers", "peers": store.PeerNodeIPsFor(node)}); err != nil {
		return
	}
	for {
		var event struct {
			Event   string          `json:"event"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := conn.ReadJSON(&event); err != nil {
			log.Printf("agent disconnected node=%s", node)
			return
		}
		if event.Event != "hello" && event.Event != "snapshot" {
			continue
		}
		var snapshot topo.AgentSnapshot
		if err := json.Unmarshal(event.Payload, &snapshot); err != nil {
			continue
		}
		if event.Event == "hello" {
			store.RecordAgentHello(node, snapshot)
		} else {
			store.RecordAgentSnapshot(node, snapshot)
		}
		if err := conn.WriteJSON(map[string]any{"event": "peers", "peers": store.PeerNodeIPsFor(node)}); err != nil {
			return
		}
	}
}
