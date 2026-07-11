package topo

import (
	"reflect"
	"testing"
)

func TestAgentSystemParsing(t *testing.T) {
	originalRunPing := runPing
	defer func() { runPing = originalRunPing }()
	var targets []string
	cache := &InterfaceCache{
		indexNames: map[int]string{1: "dn42_us02", 2: "wg0", 3: "dn42_peer"},
		addresses: map[string][]string{"dn42_dummy": {
			"172.23.70.36/32",
			"172.23.70.42/32",
			"fd6a:93d4:3358::42/128",
			"fd6a:93d4:3358::36/128",
			"fe80::606b:1cff:fe0f:65ab/64",
		}},
	}
	runPing = func(interfaceName, address string) (*float64, *float64) {
		targets = append(targets, interfaceName+" "+address)
		latency := 12.345
		loss := 10.5
		return &latency, &loss
	}

	ips := cache.CollectDN42DummyIPs()
	if !reflect.DeepEqual(ips, []string{"172.23.70.36", "fd6a:93d4:3358::36"}) {
		t.Fatalf("ips = %#v", ips)
	}
	detected := cache.CollectDN42WireGuardDetection(map[string][]string{"dn42_us02": []string{"172.23.70.35", "fd6a:93d4:3358::35"}}, []string{"172.23.70.36", "fd6a:93d4:3358::36"})
	if len(detected) != 1 || detected[0].Name != "dn42_us02" || detected[0].LatencyMS == nil || *detected[0].LatencyMS != 12.345 {
		t.Fatalf("detected = %#v", detected)
	}
	if want := []string{"dn42_us02 fe80::93d4:3358:35"}; !reflect.DeepEqual(targets, want) {
		t.Fatalf("targets = %#v", targets)
	}
}
