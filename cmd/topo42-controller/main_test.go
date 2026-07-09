package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentWSRequiresToken(t *testing.T) {
	oldToken := agentToken
	agentToken = "secret"
	defer func() { agentToken = oldToken }()

	for _, auth := range []string{"", "Bearer wrong"} {
		req := httptest.NewRequest(http.MethodGet, "/api/agent/ws?node=dn42_cn01", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()

		agentWSHandler(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("auth %q status = %d", auth, w.Code)
		}
	}
}
