package addr

import (
	"errors"
	"fmt"
	"net"
)

var (
	defaultPrivateBlocks []*net.IPNet

	ErrorInvalidAddr = errors.New("ip addr is invalid")
	ErrorIPNotFound  = errors.New("no IP address found, and explicit IP not provided")
)

func init() {
	for _, b := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10", "fd00::/8"} {
		if _, block, err := net.ParseCIDR(b); err == nil {
			defaultPrivateBlocks = append(defaultPrivateBlocks, block)
		}
	}
}

func AppendPrivateBlocks(bs ...string) {
	for _, b := range bs {
		if _, block, err := net.ParseCIDR(b); err == nil {
			defaultPrivateBlocks = append(defaultPrivateBlocks, block)
		}
	}
}

func isPrivateIP(ipAddr string) bool {
	ip := net.ParseIP(ipAddr)

	for _, priv := range defaultPrivateBlocks {
		if priv.Contains(ip) {
			return true
		}
	}

	return false
}

func IsLocal(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		addr = host
	}

	if addr == "localhost" {
		return true
	}

	for _, ip := range IPs() {
		if addr == ip {
			return true
		}
	}

	return false
}

func Extract(addr string) (string, error) {
	if len(addr) > 0 && (addr != "0.0.0.0" && addr != "[::]" && addr != "::") {
		return addr, nil
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get interfaces, err: %w", err)
	}

	addrs := []net.Addr{}
	loAddrs := []net.Addr{}

	for _, iface := range ifaces {
		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		if iface.Flags&net.FlagLoopback != 0 {
			loAddrs = append(loAddrs, ifaceAddrs...)

			continue
		}

		addrs = append(addrs, ifaceAddrs...)
	}

	addrs = append(addrs, loAddrs...)

	var ipAddr, publicIP string

	for _, rawAddr := range addrs {
		var ip net.IP
		switch addr := rawAddr.(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.IPNet:
			ip = addr.IP
		default:
			continue
		}

		if !isPrivateIP(ip.String()) {
			publicIP = ip.String()

			continue
		}

		ipAddr = ip.String()

		break
	}

	if len(ipAddr) > 0 {
		a := net.ParseIP(ipAddr)
		if a == nil {
			return "", fmt.Errorf("faild to parse addr %s, %w", ipAddr, ErrorInvalidAddr)
		}

		return a.String(), nil
	}

	if len(publicIP) > 0 {
		a := net.ParseIP(publicIP)
		if a == nil {
			return "", fmt.Errorf("faild to parse addr %s, %w", publicIP, ErrorInvalidAddr)
		}

		return a.String(), nil
	}

	return "", ErrorIPNotFound
}

func IPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var ipAddrs []string

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			// dont skip ipv6 addrs
			/*
				ip = ip.To4()
				if ip == nil {
					continue
				}
			*/

			ipAddrs = append(ipAddrs, ip.String())
		}
	}

	return ipAddrs
}
