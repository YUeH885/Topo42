package topo

import (
	"slices"
	"testing"
)

func TestAgentSystemParsing(t *testing.T) {
	originalRunPing := runPing
	defer func() { runPing = originalRunPing }()
	var targets []string
	values := []string{
		"172.23.70.36/32",
		"172.23.70.42/32",
		"fd6a:93d4:3358::42/128",
		"fd6a:93d4:3358::36/128",
		"fe80::606b:1cff:fe0f:65ab/64",
	}
	runPing = func(interfaceName, address string) (*float64, *float64) {
		targets = append(targets, interfaceName+" "+address)
		latency := 12.345
		loss := 10.5
		return &latency, &loss
	}

	ips := []string{}
	for _, value := range values {
		if ip := nodeIP(value); ip != "" {
			ips = append(ips, ip)
		}
	}
	if !slices.Equal(ips, []string{"172.23.70.36", "fd6a:93d4:3358::36"}) {
		t.Fatalf("ips = %#v", ips)
	}
	latency, _ := pingStats("dn42_us02", []string{"172.23.70.35", "fd6a:93d4:3358::35"}, ips)
	if latency == nil || *latency != 12.345 {
		t.Fatalf("latency = %#v", latency)
	}
	if want := []string{"dn42_us02 fe80::93d4:3358:35"}; !slices.Equal(targets, want) {
		t.Fatalf("targets = %#v", targets)
	}
}
