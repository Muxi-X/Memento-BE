package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

const (
	defaultHTTPAddr  = ":80"
	defaultHTTPSAddr = ":443"
	defaultCacheDir  = "/app/autocert"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	domain := strings.TrimSpace(os.Getenv("GATEWAY_DOMAIN"))
	if domain == "" {
		log.Fatal("GATEWAY_DOMAIN is required")
	}
	backendURL := strings.TrimSpace(os.Getenv("GATEWAY_BACKEND_URL"))
	if backendURL == "" {
		backendURL = "http://api:8080"
	}
	cacheDir := strings.TrimSpace(os.Getenv("GATEWAY_CACHE_DIR"))
	if cacheDir == "" {
		cacheDir = defaultCacheDir
	}
	contactEmail := strings.TrimSpace(os.Getenv("GATEWAY_EMAIL"))
	if contactEmail == "" {
		// Let’s Encrypt allows issuing without a contact email, but
		// we keep it optional to avoid blocking test deployments.
		contactEmail = ""
	}

	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		log.Fatalf("create cache dir: %v", err)
	}

	backend, err := url.Parse(backendURL)
	if err != nil {
		log.Fatalf("parse GATEWAY_BACKEND_URL: %v", err)
	}

	proxy := newReverseProxy(backend)

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		HostPolicy: autocert.HostWhitelist(domain),
		Email:      contactEmail,
	}

	// Use the manager's TLS config so the ACME ALPN protocol (acme-tls/1)
	// is enabled when Let's Encrypt chooses tls-alpn-01 challenges.
	tlsCfg := m.TLSConfig()
	tlsCfg.MinVersion = tls.VersionTLS12
	// Some clients (or proxies) may drop SNI, or send an IP address as ServerName.
	// autocert requires a hostname to select a certificate; fallback to the only
	// allowed domain so these clients don't see connection resets.
	origGetCert := tlsCfg.GetCertificate
	tlsCfg.GetCertificate = func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if chi == nil {
			return origGetCert(chi)
		}
		sni := strings.TrimSpace(chi.ServerName)
		if sni == "" || net.ParseIP(sni) != nil {
			chi2 := *chi
			chi2.ServerName = domain
			if chi.Conn != nil {
				log.Printf("gateway: missing/invalid SNI from %s, falling back to %s", chi.Conn.RemoteAddr().String(), domain)
			}
			return origGetCert(&chi2)
		}
		return origGetCert(chi)
	}

	httpsSrv := &http.Server{
		Addr:              defaultHTTPSAddr,
		Handler:           proxy,
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         tlsCfg,
	}

	// HTTP server for ACME HTTP-01 challenge and HTTPS redirect.
	redirectToHTTPS := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + domain + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	})
	httpSrv := &http.Server{
		Addr:              defaultHTTPAddr,
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           m.HTTPHandler(redirectToHTTPS),
	}

	errCh := make(chan error, 2)
	go func() {
		log.Printf("gateway: listening on %s for ACME/redirect (domain=%s)", httpSrv.Addr, domain)
		if err := httpSrv.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()
	go func() {
		log.Printf("gateway: listening on %s for HTTPS reverse proxy -> %s (domain=%s)", httpsSrv.Addr, redactedURLString(backend), domain)
		if err := httpsSrv.ListenAndServeTLS("", ""); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		_ = httpsSrv.Shutdown(shutdownCtx)
		return
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		log.Fatal(err)
	}
}

func newReverseProxy(backend *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(backend)

	// NewSingleHostReverseProxy sets a default Director.
	// Override it here (and DO NOT set Rewrite) to keep Go 1.26+ happy.
	proxy.Director = func(req *http.Request) {
		forwardedHost := req.Host

		// Keep req.URL.Path / RawPath / RawQuery intact.
		req.URL.Scheme = backend.Scheme
		req.URL.Host = backend.Host
		req.Host = backend.Host

		req.Header.Set("X-Forwarded-Host", forwardedHost)
		req.Header.Set("X-Forwarded-Proto", "https")
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// Simple, predictable error response for a gateway.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(w, "bad gateway\n")
		log.Printf("gateway: proxy error: method=%s path=%s err=%v", r.Method, r.URL.Path, err)
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		// Ensure caches don't pin test responses via gateway.
		if resp.Header.Get("Cache-Control") == "" {
			resp.Header.Set("Cache-Control", "no-store")
		}
		return nil
	}

	return proxy
}

func redactedURLString(u *url.URL) string {
	if u == nil {
		return ""
	}
	copy := *u
	if copy.User != nil {
		copy.User = url.UserPassword(copy.User.Username(), "redacted")
	}
	return copy.String()
}
