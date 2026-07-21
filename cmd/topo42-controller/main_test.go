package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentWSRequiresToken(t *testing.T) {
	oldToken := agentToken
	agentToken = "secret"
	defer func() { agentToken = oldToken }()

	for _, auth := range []string{"", "Bearer wrong"} {
		req := httptest.NewRequest(http.MethodGet, "/api/agent/ws?node=edge-a", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()

		agentWSHandler(w, req)

		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "invalid token") {
			t.Fatalf("auth %q response = %d %q", auth, w.Code, w.Body.String())
		}
	}
}
