package services

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const TMDBImageBaseURL = "https://image.tmdb.org/t/p/w500"

// DownloadTMDBImage downloads an image from TMDB CDN and saves it locally.
// subDir can be "posters" or "backdrops".
// tmdbPath is e.g. "/w3n7b...jpg"
// Returns relative path e.g. "/uploads/posters/w3n7b...jpg"
func DownloadTMDBImage(tmdbPath string, subDir string) (string, error) {
	if tmdbPath == "" {
		return "", nil
	}

	cleanPath := strings.TrimPrefix(tmdbPath, "/")
	uploadDir := filepath.Join("uploads", subDir)

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	localFileName := cleanPath
	localFilePath := filepath.Join(uploadDir, localFileName)

	// If file already exists locally, reuse it
	if _, err := os.Stat(localFilePath); err == nil {
		return "/uploads/" + subDir + "/" + localFileName, nil
	}

	imageURL := TMDBImageBaseURL + tmdbPath
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image from TMDB: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("TMDB image request failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(localFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create local image file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save image content: %v", err)
	}

	return "/uploads/" + subDir + "/" + localFileName, nil
}
