package controllers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"

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

func GetMediaDetail(c *fiber.Ctx) error {
	idStr := c.Query("id")
	if idStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Parameter 'id' wajib diisi"})
	}

	mediaType := c.Query("type", "movie")
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY belum dikonfigurasi"})
	}

	var tmdbURL string
	if mediaType == "tv" {
		tmdbURL = "https://api.themoviedb.org/3/tv/" + idStr + "?append_to_response=credits&api_key=" + apiKey
	} else {
		tmdbURL = "https://api.themoviedb.org/3/movie/" + idStr + "?append_to_response=credits&api_key=" + apiKey
	}

	resp, err := http.Get(tmdbURL)
	if err != nil || resp.StatusCode != 200 {
		return c.Status(502).JSON(fiber.Map{"error": "Gagal mengambil detail dari TMDB"})
	}
	defer resp.Body.Close()

	var detail models.TMDBDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membaca detail dari TMDB"})
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
			"total_seasons":      detail.NumberOfSeasons,
			"total_episodes":     detail.NumberOfEpisodes,
			"next_air_date":      nextAirDate,
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
	idStr := c.Query("id")
	seasonStr := c.Query("season", "1")

	if idStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Parameter 'id' wajib diisi"})
	}

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY belum dikonfigurasi"})
	}

	tmdbURL := "https://api.themoviedb.org/3/tv/" + idStr + "/season/" + seasonStr + "?api_key=" + apiKey

	resp, err := http.Get(tmdbURL)
	if err != nil || resp.StatusCode != 200 {
		return c.Status(502).JSON(fiber.Map{"error": "Gagal mengambil daftar episode dari TMDB"})
	}
	defer resp.Body.Close()

	var seasonResp TMDBSeasonResponse
	if err := json.NewDecoder(resp.Body).Decode(&seasonResp); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membaca daftar episode dari TMDB"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    seasonResp.Episodes,
	})
}