// Package netutil provides shared network utilities for HTTP clients.
package netutil

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// NewTransport creates an http.Transport with optional proxy support.
// If proxyURL is empty, returns a default transport.
// Supports http://, https://, and socks5:// proxy protocols.
func NewTransport(proxyURL string) *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	if proxyURL == "" {
		return transport
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return transport
	}

	switch parsed.Scheme {
	case "socks5":
		// Use golang.org/x/net/proxy for SOCKS5
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, nil, proxy.Direct)
		if err != nil {
			return transport
		}
		// Wrap the SOCKS5 dialer to implement DialContext
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	default:
		// http:// or https://
		transport.Proxy = http.ProxyURL(parsed)
	}

	return transport
}

// NewHTTPClient creates an http.Client with optional proxy support.
func NewHTTPClient(proxyURL string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: NewTransport(proxyURL),
	}
}
