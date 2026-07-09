package main

import "testing"

func TestControllerWSURL(t *testing.T) {
	if got := controllerWSURL("https://controller.example:8000/", "dn42_us02"); got != "wss://controller.example:8000/api/agent/ws?node=dn42_us02" {
		t.Fatalf("url = %s", got)
	}
}
