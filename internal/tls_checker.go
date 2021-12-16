package internal

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"
)

// Expiration time of TLS certificate.
func Expiration(ctx context.Context, host string) (time.Time, error) {
	var dialer = tls.Dialer{
		Config: &tls.Config{
			// To check self-signed certs also
			InsecureSkipVerify: true, //nolint:gosec
		},
	}

	rawConn, err := dialer.DialContext(ctx, "tcp", host+":443")

	if err != nil {
		return time.Time{}, fmt.Errorf("dial %s: %w", host, err)
	}
	defer rawConn.Close()

	conn, ok := rawConn.(*tls.Conn)
	if !ok {
		return time.Time{}, nil
	}

	var min time.Time
	for i, cert := range conn.ConnectionState().PeerCertificates {
		if !cert.NotAfter.IsZero() {
			if i == 0 {
				min = cert.NotAfter
			} else if cert.NotAfter.Before(min) {
				min = cert.NotAfter
			}
		}
	}

	return min, nil
}
