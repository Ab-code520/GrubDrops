// Package netutil provides shared network utilities for HTTP clients.
package netutil

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// ProxyDialer returns a dialer function that routes TCP connections through
// the given proxy. Supports socks5:// and http:// proxies. Returns nil if
// proxyURL is empty, allowing callers to fall back to direct dialing.
func ProxyDialer(proxyURL string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if proxyURL == "" {
		return nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil
	}
	switch parsed.Scheme {
	case "socks5":
		d, err := proxy.SOCKS5("tcp", parsed.Host, nil, proxy.Direct)
		if err != nil {
			return nil
		}
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			type result struct {
				conn net.Conn
				err  error
			}
			ch := make(chan result, 1)
			go func() {
				conn, err := d.Dial(network, addr)
				ch <- result{conn, err}
			}()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case r := <-ch:
				return r.conn, r.err
			}
		}
	case "http", "https":
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Connect to the proxy
			proxyAddr := parsed.Host
			d := net.Dialer{Timeout: 30 * time.Second}
			proxyConn, err := d.DialContext(ctx, "tcp", proxyAddr)
			if err != nil {
				return nil, fmt.Errorf("dial proxy: %w", err)
			}
			// Send CONNECT request
			connectReq := &http.Request{
				Method: "CONNECT",
				URL:    &url.URL{Opaque: addr},
				Host:   addr,
				Header: make(http.Header),
			}
			if parsed.User != nil {
				password, _ := parsed.User.Password()
				connectReq.SetBasicAuth(parsed.User.Username(), password)
			}
			if err := connectReq.Write(proxyConn); err != nil {
				proxyConn.Close()
				return nil, fmt.Errorf("write CONNECT: %w", err)
			}
			// Read response
			br := bufio.NewReader(proxyConn)
			resp, err := http.ReadResponse(br, connectReq)
			if err != nil {
				proxyConn.Close()
				return nil, fmt.Errorf("read CONNECT response: %w", err)
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				proxyConn.Close()
				return nil, fmt.Errorf("CONNECT failed: %s", resp.Status)
			}
			return proxyConn, nil
		}
	default:
		return nil
	}
}

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
		// Wrap the SOCKS5 dialer to implement DialContext with context support.
		// Run the dial in a goroutine so ctx cancellation is respected.
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			type result struct {
				conn net.Conn
				err  error
			}
			ch := make(chan result, 1)
			go func() {
				conn, err := dialer.Dial(network, addr)
				ch <- result{conn, err}
			}()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case r := <-ch:
				return r.conn, r.err
			}
		}
	default:
		// http:// or https://
		transport.Proxy = http.ProxyURL(parsed)
	}

	return transport
}

