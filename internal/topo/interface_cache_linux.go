package topo

import (
	"encoding/binary"
	"net"
	"net/netip"
	"slices"
	"sync"
	"syscall"
)

type InterfaceCache struct {
	mu         sync.RWMutex
	indexNames map[int]string
	addresses  map[string][]string
}

func NewInterfaceCache() *InterfaceCache {
	cache := &InterfaceCache{}
	cache.scan()
	return cache
}

func (c *InterfaceCache) WatchNetlink() error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW|syscall.SOCK_CLOEXEC, syscall.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	groups := uint32(1<<uint(syscall.RTNLGRP_LINK-1)) |
		uint32(1<<uint(syscall.RTNLGRP_IPV4_IFADDR-1)) |
		uint32(1<<uint(syscall.RTNLGRP_IPV6_IFADDR-1))
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: groups}); err != nil {
		return err
	}
	buffer := make([]byte, 65536)
	for {
		n, _, err := syscall.Recvfrom(fd, buffer, 0)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		messages, err := syscall.ParseNetlinkMessage(buffer[:n])
		if err != nil {
			continue
		}
		for _, message := range messages {
			c.handleNetlinkMessage(message)
		}
	}
}

func (c *InterfaceCache) scan() {
	nextNames := map[int]string{}
	nextAddresses := map[string][]string{}
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, item := range interfaces {
			nextNames[item.Index] = item.Name
			nextAddresses[item.Name] = interfaceAddressStrings(&item)
		}
	}
	c.mu.Lock()
	c.indexNames = nextNames
	c.addresses = nextAddresses
	c.mu.Unlock()
}

func (c *InterfaceCache) handleNetlinkMessage(message syscall.NetlinkMessage) {
	switch message.Header.Type {
	case syscall.RTM_NEWLINK:
		if index, ok := netlinkLinkIndex(message); ok {
			c.refreshInterface(index)
		}
	case syscall.RTM_DELLINK:
		if index, ok := netlinkLinkIndex(message); ok {
			c.deleteInterface(index)
		}
	case syscall.RTM_NEWADDR:
		if index, address, ok := netlinkAddress(message); ok {
			c.addAddress(index, address)
		}
	case syscall.RTM_DELADDR:
		if index, address, ok := netlinkAddress(message); ok {
			c.deleteAddress(index, address)
		}
	}
}

func (c *InterfaceCache) refreshInterface(index int) {
	item, err := net.InterfaceByIndex(index)
	if err != nil {
		c.deleteInterface(index)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old := c.indexNames[index]; old != "" && old != item.Name {
		delete(c.addresses, old)
	}
	c.indexNames[index] = item.Name
	c.addresses[item.Name] = dedupe(interfaceAddressStrings(item))
}

func (c *InterfaceCache) deleteInterface(index int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old := c.indexNames[index]; old != "" {
		delete(c.addresses, old)
	}
	delete(c.indexNames, index)
}

func (c *InterfaceCache) addAddress(index int, address string) {
	name := c.nameForIndex(index)
	if name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addresses[name] = dedupe(append(c.addresses[name], address))
}

func (c *InterfaceCache) deleteAddress(index int, address string) {
	name := c.nameForIndex(index)
	if name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addresses[name] = slices.DeleteFunc(c.addresses[name], func(value string) bool { return value == address })
}

func (c *InterfaceCache) nameForIndex(index int) string {
	c.mu.RLock()
	name := c.indexNames[index]
	c.mu.RUnlock()
	if name != "" {
		return name
	}
	c.refreshInterface(index)
	c.mu.RLock()
	name = c.indexNames[index]
	c.mu.RUnlock()
	return name
}

func interfaceAddressStrings(item *net.Interface) []string {
	addrs, err := item.Addrs()
	if err != nil {
		return []string{}
	}
	result := []string{}
	for _, addr := range addrs {
		result = append(result, addr.String())
	}
	return result
}

func netlinkLinkIndex(message syscall.NetlinkMessage) (int, bool) {
	if len(message.Data) < syscall.SizeofIfInfomsg {
		return 0, false
	}
	return int(int32(binary.NativeEndian.Uint32(message.Data[4:8]))), true
}

func netlinkAddress(message syscall.NetlinkMessage) (int, string, bool) {
	if len(message.Data) < syscall.SizeofIfAddrmsg {
		return 0, "", false
	}
	family := message.Data[0]
	prefixLen := message.Data[1]
	index := int(binary.NativeEndian.Uint32(message.Data[4:8]))
	attrs, err := syscall.ParseNetlinkRouteAttr(&message)
	if err != nil {
		return 0, "", false
	}
	fallback := ""
	for _, attr := range attrs {
		if attr.Attr.Type != syscall.IFA_LOCAL && attr.Attr.Type != syscall.IFA_ADDRESS {
			continue
		}
		address, ok := netlinkAddressString(family, prefixLen, attr.Value)
		if !ok {
			continue
		}
		if attr.Attr.Type == syscall.IFA_LOCAL {
			return index, address, true
		}
		fallback = address
	}
	return index, fallback, fallback != ""
}

func netlinkAddressString(family, prefixLen uint8, value []byte) (string, bool) {
	length := 0
	if family == syscall.AF_INET {
		length = 4
	} else if family == syscall.AF_INET6 {
		length = 16
	}
	if len(value) < length {
		return "", false
	}
	addr, ok := netip.AddrFromSlice(value[:length])
	if !ok {
		return "", false
	}
	return netip.PrefixFrom(addr, int(prefixLen)).String(), true
}
