package topo

import (
	"encoding/binary"
	"net/netip"
	"reflect"
	"syscall"
	"testing"
)

func TestInterfaceCacheAddressEvents(t *testing.T) {
	cache := &InterfaceCache{
		indexNames: map[int]string{42: "dn42_dummy"},
		addresses:  map[string][]string{"dn42_dummy": []string{"172.23.70.36/32"}},
	}
	addr := netip.MustParseAddr("fd6a:93d4:3358::36").As16()

	cache.handleNetlinkMessage(testAddrMessage(syscall.RTM_NEWADDR, syscall.AF_INET6, 42, 128, syscall.IFA_LOCAL, addr[:]))
	if got, want := cache.addresses["dn42_dummy"], []string{"172.23.70.36/32", "fd6a:93d4:3358::36/128"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("addresses after add = %#v", got)
	}

	cache.handleNetlinkMessage(testAddrMessage(syscall.RTM_DELADDR, syscall.AF_INET6, 42, 128, syscall.IFA_LOCAL, addr[:]))
	if got, want := cache.addresses["dn42_dummy"], []string{"172.23.70.36/32"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("addresses after delete = %#v", got)
	}
}

func testAddrMessage(messageType uint16, family uint8, index int, prefixLen uint8, attrType uint16, value []byte) syscall.NetlinkMessage {
	data := make([]byte, syscall.SizeofIfAddrmsg)
	data[0] = family
	data[1] = prefixLen
	binary.NativeEndian.PutUint32(data[4:8], uint32(index))
	data = append(data, testRouteAttr(attrType, value)...)
	return syscall.NetlinkMessage{Header: syscall.NlMsghdr{Type: messageType}, Data: data}
}

func testRouteAttr(attrType uint16, value []byte) []byte {
	length := syscall.SizeofRtAttr + len(value)
	data := make([]byte, (length+syscall.RTA_ALIGNTO-1)&^(syscall.RTA_ALIGNTO-1))
	binary.NativeEndian.PutUint16(data[0:2], uint16(length))
	binary.NativeEndian.PutUint16(data[2:4], attrType)
	copy(data[4:], value)
	return data
}
