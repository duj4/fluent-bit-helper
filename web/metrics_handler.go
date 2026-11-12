package web

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"fluent-bit-helper/analyzer"
	"github.com/gin-gonic/gin"
)

const maxJSONFile = 8 << 20
const maxUpload = (16 << 20) + (64 << 10)

func metricsHandler(engine *analyzer.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Streaming multipart avoids ParseMultipartForm's disk spill behavior.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUpload)
		reader, err := c.Request.MultipartReader()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Upload JSON files as multipart/form-data using dashboards and/or alert_rules."})
			return
		}
		files := map[string][]byte{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				uploadError(c, err)
				return
			}
			name := part.FormName()
			if (name != "dashboards" && name != "alert_rules") || files[name] != nil || !strings.EqualFold(filepath.Ext(part.FileName()), ".json") {
				part.Close()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Provide at most one .json file per dashboards or alert_rules field."})
				return
			}
			data, err := io.ReadAll(io.LimitReader(part, maxJSONFile+1))
			part.Close()
			if err != nil {
				uploadError(c, err)
				return
			}
			if len(data) > maxJSONFile {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Each JSON file must be at most 8 MiB."})
				return
			}
			if len(data) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "The uploaded JSON file is empty."})
				return
			}
			files[name] = data
		}
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Upload at least one Dashboard or Alert Rule JSON file."})
			return
		}
		report := engine.Analyze(files["dashboards"], files["alert_rules"])
		// Do not log payloads, queries, file contents or data-source credentials.
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, report)
	}
}

func uploadError(c *gin.Context, err error) {
	var sizeError *http.MaxBytesError
	if errors.As(err, &sizeError) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Upload exceeds the 16 MiB total request limit."})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "The multipart upload is incomplete or invalid."})
}
