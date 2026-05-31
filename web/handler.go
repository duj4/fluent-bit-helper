package web

import (
	"net/http"

	"flbctl/config"

	"github.com/gin-gonic/gin"
)

type GenerateResponse struct {
	YAML     string   `json:"yaml"`
	Warnings []string `json:"warnings,omitempty"`
}

func registerAPI(r *gin.Engine) {
	r.POST("/api/generate", generateHandler)
}

func generateHandler(c *gin.Context) {
	var input config.UserInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	config.ApplyModeDefaults(&input)

	cfg, err := config.Build(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	data, err := config.ToYAML(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, GenerateResponse{
		YAML:     string(data),
		Warnings: config.BuildWarnings(input),
	})
}
