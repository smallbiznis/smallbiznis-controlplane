package dns

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

// VerifyDNSRecord checks if the given TXT record contains the expected verification code.
// It first tries via public DNS (1.1.1.1 / 8.8.8.8), then falls back to system resolver.
func VerifyDNSRecord(hostname, expectedCode string) error {
	if strings.TrimSpace(hostname) == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	if strings.TrimSpace(expectedCode) == "" {
		return fmt.Errorf("expectedCode cannot be empty")
	}

	// normalize hostname to FQDN
	host := dns.Fqdn(hostname)
	zap.L().Debug("Verifying DNS TXT record", zap.String("host", host))

	// try via miekg/dns (public resolvers)
	publicResolvers := []string{"1.1.1.1:53", "8.8.8.8:53"}
	for _, resolver := range publicResolvers {
		if err := queryTXTWithResolver(host, expectedCode, resolver); err == nil {
			zap.L().Info("DNS TXT verification success", zap.String("resolver", resolver), zap.String("hostname", hostname))
			return nil
		}
	}

	// fallback to system resolver
	zap.L().Warn("Falling back to system resolver", zap.String("hostname", hostname))
	if err := queryTXTSystem(host, expectedCode); err == nil {
		zap.L().Info("DNS TXT verification success (system resolver)", zap.String("hostname", hostname))
		return nil
	}

	return fmt.Errorf("no matching TXT record found for %s", hostname)
}

// queryTXTWithResolver uses a specific DNS resolver for TXT lookup
func queryTXTWithResolver(hostname, expectedCode, resolver string) error {
	client := &dns.Client{
		Timeout: 3 * time.Second,
	}

	msg := dns.Msg{}
	msg.SetQuestion(hostname, dns.TypeTXT)

	resp, _, err := client.Exchange(&msg, resolver)
	if err != nil {
		zap.L().Debug("DNS query failed", zap.String("resolver", resolver), zap.Error(err))
		return err
	}

	for _, ans := range resp.Answer {
		if txt, ok := ans.(*dns.TXT); ok {
			for _, record := range txt.Txt {
				if strings.TrimSpace(record) == expectedCode {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("no matching TXT record found at resolver %s", resolver)
}

// queryTXTSystem uses Go's standard net.LookupTXT for fallback
func queryTXTSystem(hostname, expectedCode string) error {
	records, err := net.LookupTXT(hostname)
	if err != nil {
		return fmt.Errorf("system resolver TXT lookup failed: %w", err)
	}

	for _, r := range records {
		if strings.TrimSpace(r) == expectedCode {
			return nil
		}
	}

	return fmt.Errorf("no matching TXT record found via system resolver")
}
