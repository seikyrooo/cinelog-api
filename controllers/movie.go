package controllers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"cinelog-api/database"
	"cinelog-api/models"

	"github.com/gofiber/fiber/v2"
)

var tmdbHttpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func sanitizeTMDBResults(results []models.TMDBMedia, defaultType string) []models.TMDBMedia {
	var filtered []models.TMDBMedia
	for _, item := range results {
		if item.MediaType == "" {
			if defaultType != "" && defaultType != "all" {
				item.MediaType = defaultType
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
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// GetTrending returns trending movies/TV shows from TMDB
func GetTrending(c *fiber.Ctx) error {
	mediaType := c.Query("type", "all")
	timeWindow := c.Query("time", "week")

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY is not configured"})
	}

	if timeWindow != "day" && timeWindow != "week" {
		timeWindow = "week"
	}

	validType := "all"
	if mediaType == "movie" || mediaType == "tv" {
		validType = mediaType
	}

	tmdbURL := "https://api.themoviedb.org/3/trending/" + validType + "/" + timeWindow + "?api_key=" + apiKey

	resp, err := tmdbHttpClient.Get(tmdbURL)
	if err != nil || resp.StatusCode != 200 {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to fetch trending data from TMDB"})
	}
	defer resp.Body.Close()

	var tmdbResp models.TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&tmdbResp); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse TMDB response"})
	}

	filteredResults := sanitizeTMDBResults(tmdbResp.Results, mediaType)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    filteredResults,
	})
}

// GetDiscover returns top popular or highest rated content
func GetDiscover(c *fiber.Ctx) error {
	mediaType := c.Query("type", "movie")
	sortBy := c.Query("sort", "popularity.desc")

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY is not configured"})
	}

	endpoint := "movie"
	if mediaType == "tv" {
		endpoint = "tv"
	}

	tmdbURL := "https://api.themoviedb.org/3/discover/" + endpoint + "?sort_by=" + sortBy + "&vote_count.gte=100&api_key=" + apiKey

	resp, err := tmdbHttpClient.Get(tmdbURL)
	if err != nil || resp.StatusCode != 200 {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to fetch discover list from TMDB"})
	}
	defer resp.Body.Close()

	var tmdbResp models.TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&tmdbResp); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse TMDB discover response"})
	}

	filteredResults := sanitizeTMDBResults(tmdbResp.Results, endpoint)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    filteredResults,
	})
}

func SearchMovies(c *fiber.Ctx) error {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Search query parameter 'q' is required"})
	}

	mediaType := c.Query("type", "all")

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY is not configured"})
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

	resp, err := tmdbHttpClient.Get(tmdbURL)
	if err != nil || resp.StatusCode != 200 {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to fetch search results from TMDB"})
	}
	defer resp.Body.Close()

	var tmdbResp models.TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&tmdbResp); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse TMDB response"})
	}

	filteredResults := sanitizeTMDBResults(tmdbResp.Results, mediaType)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    filteredResults,
	})
}

func GetMediaDetail(c *fiber.Ctx) error {
	idStr := strings.TrimSpace(c.Query("id"))
	if idStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Parameter 'id' is required"})
	}

	mediaType := c.Query("type", "movie")
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY is not configured"})
	}

	var tmdbURL string
	if mediaType == "tv" {
		tmdbURL = "https://api.themoviedb.org/3/tv/" + idStr + "?append_to_response=credits&api_key=" + apiKey
	} else {
		tmdbURL = "https://api.themoviedb.org/3/movie/" + idStr + "?append_to_response=credits&api_key=" + apiKey
	}

	resp, err := tmdbHttpClient.Get(tmdbURL)
	if err != nil || resp.StatusCode != 200 {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to fetch media details from TMDB"})
	}
	defer resp.Body.Close()

	var detail models.TMDBDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse media details from TMDB"})
	}

	var director string
	if mediaType == "tv" {
		var creators []string
		for _, creator := range detail.CreatedBy {
			if creator.Name != "" {
				creators = append(creators, creator.Name)
			}
		}
		if len(creators) > 0 {
			director = strings.Join(creators, ", ")
		} else if detail.Credits != nil {
			for _, crew := range detail.Credits.Crew {
				if crew.Job == "Executive Producer" || crew.Job == "Director" || crew.Job == "Writer" {
					director = crew.Name
					break
				}
			}
		}
	} else if detail.Credits != nil {
		for _, crew := range detail.Credits.Crew {
			if crew.Job == "Director" {
				director = crew.Name
				break
			}
		}
	}

	var topCast []string
	if detail.Credits != nil {
		for i, cast := range detail.Credits.Cast {
			if i >= 5 {
				break
			}
			topCast = append(topCast, cast.Name)
		}
	}
	castStr := strings.Join(topCast, ", ")

	title := detail.Title
	if title == "" {
		title = detail.Name
	}
	releaseDate := detail.ReleaseDate
	if releaseDate == "" {
		releaseDate = detail.FirstAirDate
	}

	var nextAirDate string
	var nextEpsName string
	if detail.NextEpisodeToAir != nil {
		nextAirDate = detail.NextEpisodeToAir.AirDate
		nextEpsName = detail.NextEpisodeToAir.Name
	}

	var seasonsList []models.TMDBSeasonItem
	for _, s := range detail.Seasons {
		if s.SeasonNumber > 0 {
			seasonsList = append(seasonsList, s)
		}
	}

	// Auto-heal/sync movie in local database if it already exists with missing metadata
	go func(tmdbID int, poster, backdrop, overview, relDate, status string, seasons, episodes int) {
		var m models.Movie
		if err := database.DB.Where("tmdb_id = ?", tmdbID).First(&m).Error; err == nil {
			updated := false
			if poster != "" && (m.PosterPath == "" || m.PosterPath != poster) {
				m.PosterPath = poster
				updated = true
			}
			if backdrop != "" && (m.BackdropPath == "" || m.BackdropPath != backdrop) {
				m.BackdropPath = backdrop
				updated = true
			}
			if overview != "" && m.Overview == "" {
				m.Overview = overview
				updated = true
			}
			if seasons > 0 && m.TotalSeasons == 0 {
				m.TotalSeasons = seasons
				updated = true
			}
			if episodes > 0 && m.TotalEpisodes == 0 {
				m.TotalEpisodes = episodes
				updated = true
			}
			if status != "" && m.MediaStatus == "" {
				m.MediaStatus = status
				updated = true
			}
			if updated {
				database.DB.Save(&m)
			}
		}
	}(detail.ID, detail.PosterPath, detail.BackdropPath, detail.Overview, releaseDate, detail.Status, detail.NumberOfSeasons, detail.NumberOfEpisodes)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":                detail.ID,
			"title":             title,
			"overview":          detail.Overview,
			"poster_path":       detail.PosterPath,
			"backdrop_path":     detail.BackdropPath,
			"release_date":      releaseDate,
			"vote_average":      detail.VoteAverage,
			"media_type":        mediaType,
			"director":          director,
			"cast":              castStr,
			"total_seasons":     detail.NumberOfSeasons,
			"total_episodes":    detail.NumberOfEpisodes,
			"next_air_date":     nextAirDate,
			"next_episode_name": nextEpsName,
			"media_status":      detail.Status,
			"seasons":           seasonsList,
		},
	})
}

type TMDBSeasonEpisode struct {
	AirDate       string  `json:"air_date"`
	EpisodeNumber int     `json:"episode_number"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	StillPath     string  `json:"still_path"`
	VoteAverage   float64 `json:"vote_average"`
}

type TMDBSeasonResponse struct {
	Episodes []TMDBSeasonEpisode `json:"episodes"`
}

func GetTVSeasonEpisodes(c *fiber.Ctx) error {
	idStr := strings.TrimSpace(c.Query("id"))
	seasonStr := strings.TrimSpace(c.Query("season", "1"))

	if idStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Parameter 'id' is required"})
	}

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY is not configured"})
	}

	tmdbURL := "https://api.themoviedb.org/3/tv/" + idStr + "/season/" + seasonStr + "?api_key=" + apiKey

	resp, err := tmdbHttpClient.Get(tmdbURL)
	if err != nil || resp.StatusCode != 200 {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to fetch season episodes from TMDB"})
	}
	defer resp.Body.Close()

	var seasonResp TMDBSeasonResponse
	if err := json.NewDecoder(resp.Body).Decode(&seasonResp); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse season episodes from TMDB"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    seasonResp.Episodes,
	})
}