package topo

import (
	"reflect"
	"testing"
	"time"
)

func TestAgentHelpers(t *testing.T) {
	peers, ok := PeerNodeIPsFromEvent([]byte(`{"event":"peers","peers":{"dn42_cn01":["fd6a::1"]}}`))
	if !ok || !reflect.DeepEqual(peers, map[string][]string{"dn42_cn01": []string{"fd6a::1"}}) {
		t.Fatalf("peers = %#v ok=%v", peers, ok)
	}
	if _, ok := PeerNodeIPsFromEvent([]byte(`{"event":"peers","peers":{"dn42_cn01":["fd6a::1",42]}}`)); ok {
		t.Fatal("mixed peer ip list parsed")
	}
	if _, ok := PeerNodeIPsFromEvent([]byte(`{"event":"snapshot"}`)); ok {
		t.Fatal("snapshot parsed as peer update")
	}
}

func TestAgentSystemParsing(t *testing.T) {
	original := runCommand
	defer func() { runCommand = original }()
	var commands [][]string
	runCommand = func(command []string, timeout time.Duration) commandResult {
		commands = append(commands, command)
		switch {
		case reflect.DeepEqual(command, []string{"ip", "-o", "addr", "show", "dev", "dn42_dummy"}):
			return commandResult{stdout: "" +
				"32: dn42_dummy inet 172.23.70.36/32 scope global dn42_dummy\n" +
				"32: dn42_dummy inet 172.23.70.42/32 scope global dn42_dummy\n" +
				"32: dn42_dummy inet6 fd6a:93d4:3358::42/128 scope global\n" +
				"32: dn42_dummy inet6 fd6a:93d4:3358::36/128 scope global\n" +
				"32: dn42_dummy inet6 fe80::606b:1cff:fe0f:65ab/64 scope link\n"}
		case reflect.DeepEqual(command, []string{"wg", "show", "interfaces"}):
			return commandResult{stdout: "dn42_us02 wg0 dn42_peer\n"}
		default:
			return commandResult{stdout: "" +
				"10 packets transmitted, 9 received, 10.5% packet loss, time 9009ms\n" +
				"rtt min/avg/max/mdev = 10.00/12.345/14.00/0.20 ms\n"}
		}
	}

	ips := CollectDN42DummyIPs()
	if !reflect.DeepEqual(ips, []string{"172.23.70.36", "fd6a:93d4:3358::36"}) {
		t.Fatalf("ips = %#v", ips)
	}
	detected := CollectDN42WireGuardDetection(map[string][]string{"dn42_us02": []string{"172.23.70.35", "fd6a:93d4:3358::35"}}, []string{"172.23.70.36", "fd6a:93d4:3358::36"})
	if len(detected) != 1 || detected[0].Name != "dn42_us02" || detected[0].LatencyMS == nil || *detected[0].LatencyMS != 12.345 {
		t.Fatalf("detected = %#v", detected)
	}
	if want := []string{"ping", "-c", "10", "-W", "1", "-I", "dn42_us02", "fe80::93d4:3358:35"}; !reflect.DeepEqual(commands[len(commands)-1], want) {
		t.Fatalf("last command = %#v", commands[len(commands)-1])
	}
}
