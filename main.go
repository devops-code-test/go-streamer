package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Configuration constants
const (
	UploadFolder     = "uploads"
	OutputFolder     = "streams"
	MaxContentLength = 100 * 1024 * 1024 // 100MB max upload size
)

var AllowedExtensions = map[string]bool{
	"mp4": true, "avi": true, "mov": true,
	"mkv": true, "wmv": true, "flv": true, "webm": true,
}

func main() {
	// Create necessary directories
	os.MkdirAll(UploadFolder, os.ModePerm)
	os.MkdirAll(OutputFolder, os.ModePerm)

	// Set up Gin router
	router := gin.Default()

	// Enable CORS
	router.Use(cors.Default())

	// Serve static files and templates
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./static")

	// Routes
	router.GET("/", indexHandler)
	router.POST("/upload", uploadFileHandler)
	router.GET("/stream/:videoID/:formatType/*filename", streamFileHandler)
	router.GET("/player/:videoID", playerHandler)
	router.GET("/videos", videoListHandler)

	// Start the server
	log.Println("Starting server on port 5000...")
	router.Run(":5000")
}

func indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

func uploadFileHandler(c *gin.Context) {
	// Get the file from form data
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file part"})
		return
	}
	defer file.Close()

	if header.Filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No selected file"})
		return
	}

	// Check if file type is allowed
	if !isAllowedFile(header.Filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type not allowed"})
		return
	}

	// Generate a unique ID for this video
	videoID := uuid.New().String()

	// Create directories for this video
	videoUploadDir := filepath.Join(UploadFolder, videoID)
	os.MkdirAll(videoUploadDir, os.ModePerm)

	streamOutputDir := filepath.Join(OutputFolder, videoID)
	os.MkdirAll(streamOutputDir, os.ModePerm)

	// Save the original file
	filename := secureFilename(header.Filename)
	filePath := filepath.Join(videoUploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer out.Close()

	// Copy the file data to the file system
	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	log.Printf("File uploaded: %s", filePath)

	// Create directories for each format
	hlsOutputDir := filepath.Join(streamOutputDir, "hls")
	dashOutputDir := filepath.Join(streamOutputDir, "dash")

	// Process video (non-async for simplicity, but could be made async with goroutines)
	hlsResult := convertToHLS(filePath, hlsOutputDir)
	dashResult := convertToDASH(filePath, dashOutputDir)

	if hlsResult && dashResult {
		c.JSON(http.StatusOK, gin.H{
			"id":         videoID,
			"status":     "success",
			"hls_url":    fmt.Sprintf("/stream/%s/hls/playlist.m3u8", videoID),
			"dash_url":   fmt.Sprintf("/stream/%s/dash/manifest.mpd", videoID),
			"player_url": fmt.Sprintf("/player/%s", videoID),
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Conversion failed"})
	}
}

func streamFileHandler(c *gin.Context) {
	videoID := c.Param("videoID")
	formatType := c.Param("formatType")
	filename := c.Param("filename")

	// Remove leading slash from filename
	filename = strings.TrimPrefix(filename, "/")

	directory := filepath.Join(OutputFolder, videoID, formatType)
	filePath := filepath.Join(directory, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filePath)
}

func playerHandler(c *gin.Context) {
	videoID := c.Param("videoID")
	hlsURL := fmt.Sprintf("/stream/%s/hls/playlist.m3u8", videoID)
	dashURL := fmt.Sprintf("/stream/%s/dash/manifest.mpd", videoID)

	c.HTML(http.StatusOK, "player.html", gin.H{
		"video_id": videoID,
		"hls_url":  hlsURL,
		"dash_url": dashURL,
	})
}

func videoListHandler(c *gin.Context) {
	videos := []map[string]interface{}{}

	// Get all subdirectories in the streams folder
	entries, err := os.ReadDir(OutputFolder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read videos directory"})
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			videoID := entry.Name()
			videoDir := filepath.Join(OutputFolder, videoID)

			hlsPath := filepath.Join(videoDir, "hls", "playlist.m3u8")
			dashPath := filepath.Join(videoDir, "dash", "manifest.mpd")

			hlsExists := fileExists(hlsPath)
			dashExists := fileExists(dashPath)

			if hlsExists || dashExists {
				video := map[string]interface{}{
					"id":         videoID,
					"player_url": fmt.Sprintf("/player/%s", videoID),
				}

				if hlsExists {
					video["hls_url"] = fmt.Sprintf("/stream/%s/hls/playlist.m3u8", videoID)
				} else {
					video["hls_url"] = nil
				}

				if dashExists {
					video["dash_url"] = fmt.Sprintf("/stream/%s/dash/manifest.mpd", videoID)
				} else {
					video["dash_url"] = nil
				}

				videos = append(videos, video)
			}
		}
	}

	c.JSON(http.StatusOK, videos)
}

// Helper functions

func isAllowedFile(filename string) bool {
	ext := filepath.Ext(filename)
	if ext == "" {
		return false
	}
	ext = strings.ToLower(ext[1:]) // Remove dot and convert to lowercase
	return AllowedExtensions[ext]
}

func secureFilename(filename string) string {
	// Simple implementation to make filename safe
	return strings.ReplaceAll(filename, " ", "_")
}

func convertToHLS(inputPath, outputDir string) bool {
	os.MkdirAll(outputDir, os.ModePerm)

	hlsPlaylist := filepath.Join(outputDir, "playlist.m3u8")

	// HLS conversion command
	cmd := exec.Command(
		"ffmpeg", "-i", inputPath,
		"-profile:v", "baseline",
		"-level", "3.0",
		"-start_number", "0",
		"-hls_time", "10",
		"-hls_list_size", "0",
		"-f", "hls",
		hlsPlaylist,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("HLS conversion failed: %v\nOutput: %s", err, output)
		return false
	}

	log.Printf("HLS conversion completed for %s", inputPath)
	return true
}

func convertToDASH(inputPath, outputDir string) bool {
	os.MkdirAll(outputDir, os.ModePerm)

	dashPlaylist := filepath.Join(outputDir, "manifest.mpd")

	// DASH conversion command
	cmd := exec.Command(
		"ffmpeg", "-i", inputPath,
		"-map", "0:v", "-map", "0:a",
		"-c:v", "libx264", "-x264-params", "keyint=60:min-keyint=60:no-scenecut=1",
		"-b:v:0", "1500k",
		"-c:a", "aac", "-b:a", "128k",
		"-bf", "1", "-keyint_min", "60",
		"-g", "60", "-sc_threshold", "0",
		"-f", "dash",
		"-use_template", "1", "-use_timeline", "1",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		"-adaptation_sets", "id=0,streams=v id=1,streams=a",
		dashPlaylist,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("DASH conversion failed: %v\nOutput: %s", err, output)
		return false
	}

	log.Printf("DASH conversion completed for %s", inputPath)
	return true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
