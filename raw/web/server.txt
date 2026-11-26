package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"fluent-bit-helper/analyzer"
	migrationrules "fluent-bit-helper/migration-rules"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFiles embed.FS

const listenAddr = ":8443"

// NewRouter builds the tool pages and APIs with one verified migration rule set.
func NewRouter() (*gin.Engine, error) {
	engine, err := analyzer.New()
	if err != nil {
		return nil, err
	}
	r := gin.Default()
	registerAPI(r)
	r.POST("/api/metrics-analyzer/analyze", metricsHandler(engine))
	// Serve the exact embedded bytes verified by analyzer.New above.
	rulesJSON := migrationrules.JSON()
	r.Match([]string{http.MethodGet, http.MethodHead}, "/api/metrics-analyzer/rules", func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, "application/json; charset=utf-8", rulesJSON)
	})

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}

	r.StaticFS("/static", http.FS(staticFS))
	for path, file := range map[string]string{"/": "index.html", "/metrics-analyzer": "metrics.html", "/metrics-analyzer/rules": "rules.html"} {
		content, err := fs.ReadFile(staticFS, file)
		if err != nil {
			return nil, err
		}
		// Serve page bytes explicitly: http.FileServer redirects index.html.
		r.Match([]string{http.MethodGet, http.MethodHead}, path, func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", content) })
	}
	// Preserve the old asset URLs for existing links without a root catch-all.
	r.StaticFileFS("/style.css", "style.css", http.FS(staticFS))
	r.StaticFileFS("/app.js", "app.js", http.FS(staticFS))
	return r, nil
}

// Run starts the tool service over TLS using certificates from the working directory.
func Run() error {
	r, err := NewRouter()
	if err != nil {
		return err
	}

	certFilePath, keyFilePath, err := loadTLSFiles()
	if err != nil {
		return err
	}

	return r.RunTLS(listenAddr, certFilePath, keyFilePath)
}

// loadTLSFiles loads TLS related files from working directory
func loadTLSFiles() (string, string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get working directory: %w", err)
	}

	certFilePath := filepath.Join(workingDir, "tls", "tls.pem")
	keyFilePath := filepath.Join(workingDir, "tls", "tls.key")

	if _, err := os.Stat(certFilePath); err != nil {
		return "", "", fmt.Errorf("TLS certificate file not found: %s: %w", certFilePath, err)
	}

	if _, err := os.Stat(keyFilePath); err != nil {
		return "", "", fmt.Errorf("TLS key file not found: %s: %w", keyFilePath, err)
	}

	return certFilePath, keyFilePath, nil
}
