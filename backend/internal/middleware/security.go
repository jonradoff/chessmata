package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
)

// GAInlineScript returns the inline script body for Google Analytics.
// This must be used by all injection points so the CSP hash matches.
func GAInlineScript(gaID string) string {
	return fmt.Sprintf("window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments);}gtag('js',new Date());gtag('config','%s');", gaID)
}

// SecurityHeaders adds security-related HTTP headers to all responses.
func SecurityHeaders(googleAnalyticsID string) func(http.Handler) http.Handler {
	// Build CSP directives
	scriptSrc := "'self' 'wasm-unsafe-eval'"
	imgSrc := "'self' data: blob:"
	connectSrc := "'self' wss: ws: blob: https://raw.githack.com https://raw.githubusercontent.com"

	if googleAnalyticsID != "" {
		// Compute sha256 hash of the inline GA script for CSP
		scriptBody := GAInlineScript(googleAnalyticsID)
		hash := sha256.Sum256([]byte(scriptBody))
		hashStr := base64.StdEncoding.EncodeToString(hash[:])

		scriptSrc += " https://www.googletagmanager.com 'sha256-" + hashStr + "'"
		imgSrc += " https://www.googletagmanager.com"
		connectSrc += " https://*.google-analytics.com https://*.analytics.google.com https://*.googletagmanager.com"
	}

	csp := fmt.Sprintf(
		"default-src 'self'; script-src %s; style-src 'self' 'unsafe-inline'; img-src %s; connect-src %s; font-src 'self'; worker-src 'self' blob:",
		scriptSrc, imgSrc, connectSrc,
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-XSS-Protection", "0")
			w.Header().Set("Content-Security-Policy", csp)
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			next.ServeHTTP(w, r)
		})
	}
}
