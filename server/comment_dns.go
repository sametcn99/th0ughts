package server

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

var dnsUnavailable = errors.New("comment_dns_unavailable")

type mailDNS interface {
	LookupMX(context.Context, string) ([]*net.MX, error)
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

// Check only the normalized domain. The local part never leaves the process via DNS.
func checkMailDomain(ctx context.Context, address string, resolver mailDNS) error {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	_, domain, ok := strings.Cut(address, "@")
	if !ok || domain == "" {
		return invalidEmail
	}
	domain += "." // Absolute queries avoid the host's DNS search suffixes.
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	mx, err := resolver.LookupMX(ctx, domain)
	if err != nil && !dnsNotFound(err) {
		return dnsUnavailable
	}
	if len(mx) > 0 {
		for _, record := range mx {
			if record.Host != "" && record.Host != "." {
				return nil
			}
		}
		return invalidEmail // An explicit null MX disables implicit address fallback.
	}
	ips, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil && !dnsNotFound(err) {
		return dnsUnavailable
	}
	if len(ips) == 0 {
		return invalidEmail
	}
	return nil
}

func dnsNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}
