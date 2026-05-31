package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFiles embed.FS

// const listenAddr = ":8443"
const listenAddr = ":8080"

// Run starts the Fluent-Bit config generator web service.
func Run() error {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	// r := gin.Default()

	registerAPI(r)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}

	r.StaticFS("/", http.FS(staticFS))

	// certFilePath, keyFilePath, err := loadTLSFiles()
	// if err != nil {
	// 	return err
	// }

	// return r.RunTLS(listenAddr, certFilePath, keyFilePath)
	return r.Run(listenAddr)
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
