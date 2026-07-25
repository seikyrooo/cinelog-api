package controllers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"

	"cinelog-api/models"

	"github.com/gofiber/fiber/v2"
)

func SearchMovies(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Parameter pencarian 'q' wajib diisi"})
	}

	mediaType := c.Query("type", "all")

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY belum dikonfigurasi"})
	}

	escapedQuery := url.QueryEscape(query)
	var tmdbURL string
	if mediaType == "movie" {
		tmdbURL = "https://api.themoviedb.org/3/search/movie?query=" + escapedQuery + "&api_key=" + apiKey
	} else if mediaType == "tv" {
		tmdbURL = "https://api.themoviedb.org/3/search/tv?query=" + escapedQuery + "&api_key=" + apiKey
	} else {
		tmdbURL = "https://api.themoviedb.org/3/search/multi?query=" + escapedQuery + "&api_key=" + apiKey
	}

	resp, err := http.Get(tmdbURL)
	if err != nil || resp.StatusCode != 200 {
		return c.Status(502).JSON(fiber.Map{"error": "Gagal mengambil data dari TMDB"})
	}
	defer resp.Body.Close()

	var tmdbResp models.TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&tmdbResp); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membaca respons dari TMDB"})
	}

	var filteredResults []models.TMDBMedia
	for _, item := range tmdbResp.Results {
		if item.MediaType == "" {
			if mediaType != "all" {
				item.MediaType = mediaType
			} else {
				if item.Title != "" {
					item.MediaType = "movie"
				} else if item.Name != "" {
					item.MediaType = "tv"
				}
			}
		}

		if item.MediaType == "movie" || item.MediaType == "tv" {
			if item.Title == "" && item.Name != "" {
				item.Title = item.Name
			}
			if item.ReleaseDate == "" && item.FirstAirDate != "" {
				item.ReleaseDate = item.FirstAirDate
			}
			filteredResults = append(filteredResults, item)
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    filteredResults,
	})
}