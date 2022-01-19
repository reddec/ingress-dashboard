package internal

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"
)

type CertInfo struct {
	Expiration time.Time
	Domains    []string
	Issuer     string
}

// Expiration time of TLS certificate.
func Expiration(ctx context.Context, host string) (CertInfo, error) {
	var dialer = tls.Dialer{
		Config: &tls.Config{
			// To check self-signed certs also
			InsecureSkipVerify: true, //nolint:gosec
		},
	}

	rawConn, err := dialer.DialContext(ctx, "tcp", host+":443")

	if err != nil {
		return CertInfo{}, fmt.Errorf("dial %s: %w", host, err)
	}
	defer rawConn.Close()

	conn, ok := rawConn.(*tls.Conn)
	if !ok {
		return CertInfo{}, nil
	}

	var usedDomains = make(map[string]bool)

	var min time.Time
	var issuer string
	for i, cert := range conn.ConnectionState().PeerCertificates { //nolint:varnamelen
		if !cert.NotAfter.IsZero() {
			if i == 0 {
				min = cert.NotAfter
			} else if cert.NotAfter.Before(min) {
				min = cert.NotAfter
			}
		}
		if i == 0 { // own cert should go first
			if cert.Subject.CommonName != "" {
				usedDomains[cert.Subject.CommonName] = true
			}
			for _, alt := range cert.DNSNames {
				usedDomains[alt] = true
			}
			issuer = cert.Issuer.CommonName
		}
	}

	var domains = make([]string, 0, len(usedDomains))
	for d := range usedDomains {
		domains = append(domains, d)
	}

	return CertInfo{
		Expiration: min,
		Domains:    domains,
		Issuer:     issuer,
	}, nil
}
