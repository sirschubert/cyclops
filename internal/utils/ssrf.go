package utils

import (
	"fmt"
	"net"
	"strings"
	"syscall"
)

// blockedNets are the link-local / cloud-metadata ranges most commonly abused
// for SSRF (e.g. the 169.254.169.254 instance metadata endpoint).
var blockedNets = func() []*net.IPNet {
	cidrs := []string{
		"169.254.0.0/16",    // IPv4 link-local (incl. cloud metadata 169.254.169.254)
		"fe80::/10",         // IPv6 link-local
		"fd00:ec2::254/128", // AWS IMDS over IPv6
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// IsMetadataHost reports whether host (a hostname, IP, or "host:port") refers to
// a cloud-metadata / link-local address that should not be connected to.
func IsMetadataHost(host string) bool {
	if host == "" {
		return false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	switch strings.ToLower(host) {
	case "metadata.google.internal", "metadata.goog":
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// MetadataDialControl is a net.Dialer.Control function that refuses connections
// to metadata/link-local addresses. Because it runs after DNS resolution with
// the concrete remote address, it also blocks hostnames that resolve into those
// ranges — the core SSRF defense.
func MetadataDialControl(network, address string, _ syscall.RawConn) error {
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	}
	if IsMetadataHost(host) {
		return fmt.Errorf("blocked connection to metadata/link-local address: %s", host)
	}
	return nil
}
