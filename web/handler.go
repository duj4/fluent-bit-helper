package web

import (
	"net/http"
	"strings"

	"flbctl/config"

	"github.com/gin-gonic/gin"
)

// generateResponse is returned by POST /api/generate.
type generateResponse struct {
	YAML     string   `json:"yaml"`
	Warnings []string `json:"warnings,omitempty"`
}

// doctorRequest is returned by the Web UI when users paste or upload an existing Fluent Bit YAML.
// The backend receives YAML content directly and does not read files from the server filesystem.
type doctorRequest struct {
	YAML string `json:"yaml"`
}

// registerAPI registers all JSON APIs used by the frontend.
func registerAPI(r *gin.Engine) {
	r.POST("/api/generate", generateHandler)
	r.POST("/api/doctor", doctorHandler)
}

// generateHandler builds a new Fluent Bit YAML from the Web UI form payload.
func generateHandler(c *gin.Context) {
	var req config.UserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Apply mode-specific defaults on the backend so direct API calls and UI calls behave consistently.
	config.ApplyModeDefaults(&req)

	cfg, err := config.Build(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	yamlBytes, err := config.ToYAML(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, generateResponse{
		YAML:     string(yamlBytes),
		Warnings: config.BuildWarnings(req),
	})
}

// doctorHandler checks an existing Fluent Bit YAML against the production baseline.
// It intentionally does not validate whether paths exist on the Web server, because the paths
// belong to the target Fluent Bit host rather than to the flbctl server.
func doctorHandler(c *gin.Context) {
	var req doctorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.YAML) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "yaml content is empty",
		})
		return
	}

	report, err := config.DoctorYAML([]byte(req.YAML))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, report)
}
