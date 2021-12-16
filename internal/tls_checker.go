package internal

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// Expiration time of TLS certificate.
func Expiration(ctx context.Context, host string) (time.Time, error) {
	conn, err := tls.DialWithDialer(&net.Dialer{
		Cancel: ctx.Done(),
	}, "tcp", host+":443", &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("dial %s: %w", host, err)
	}
	defer conn.Close()

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
