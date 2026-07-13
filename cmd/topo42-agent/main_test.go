package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"topo42/internal/topo"
)

func TestWebSocketSnapshotExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		if err := conn.WriteJSON(map[string][]string{}); err != nil {
			t.Error(err)
			return
		}
		var snapshot topo.AgentSnapshot
		if err := conn.ReadJSON(&snapshot); err != nil {
			t.Error(err)
			return
		}
		if snapshot.AgentVersion != topo.Version {
			t.Errorf("agent version = %q", snapshot.AgentVersion)
		}
	}))
	defer server.Close()

	serverURL := "ws" + strings.TrimPrefix(server.URL, "http")
	if err := runWebSocketClient(serverURL, "dn42_us01", "", 0); err == nil {
		t.Fatal("expected disconnect error")
	}
}
