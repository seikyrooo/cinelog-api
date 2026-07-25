package controllers

import (
	"strconv"

	"cinelog-api/database"
	"cinelog-api/models"
	"cinelog-api/services"

	"github.com/gofiber/fiber/v2"
)

type WatchlistInput struct {
	TMDBID          int     `json:"tmdb_id"`
	MediaType       string  `json:"media_type"` // "movie" or "tv"
	Title           string  `json:"title"`
	Overview        string  `json:"overview"`
	PosterPath      string  `json:"poster_path"`
	BackdropPath    string  `json:"backdrop_path"`
	ReleaseDate     string  `json:"release_date"`
	VoteAverage     float64 `json:"vote_average"`
	Status          string  `json:"status"` // plan_to_watch, watching, completed, on_hold, dropped
	Rating          float64 `json:"rating"`
	Favorite        bool    `json:"favorite"`
	Notes           string  `json:"notes"`
	EpisodesWatched int     `json:"episodes_watched"`
	TotalEpisodes   int     `json:"total_episodes"`
}

// AddToWatchlist creates or updates a movie/tv item in DB & user's list
func AddToWatchlist(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	userID := uint(userIDVal.(float64))

	var input WatchlistInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input JSON tidak valid"})
	}

	if input.TMDBID == 0 || input.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "TMDB ID dan Judul wajib diisi"})
	}

	if input.MediaType == "" {
		input.MediaType = "movie"
	}
	if input.Status == "" {
		input.Status = "plan_to_watch"
	}

	// 1. Cek apakah Media (Movie/TV) sudah ada di DB
	var movie models.Movie
	err := database.DB.Where("tmdb_id = ? AND media_type = ?", input.TMDBID, input.MediaType).First(&movie).Error
	if err != nil {
		// Belum ada di DB -> Download poster & backdrop ke local storage VPS
		localPoster, _ := services.DownloadTMDBImage(input.PosterPath, "posters")
		localBackdrop, _ := services.DownloadTMDBImage(input.BackdropPath, "backdrops")

		movie = models.Movie{
			TMDBID:            input.TMDBID,
			MediaType:         input.MediaType,
			Title:             input.Title,
			Overview:          input.Overview,
			ReleaseDate:       input.ReleaseDate,
			PosterPath:        input.PosterPath,
			BackdropPath:      input.BackdropPath,
			LocalPosterPath:   localPoster,
			LocalBackdropPath: localBackdrop,
			VoteAverage:       input.VoteAverage,
		}

		if err := database.DB.Create(&movie).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal menyimpan metadata media ke database"})
		}
	}

	// 2. Upsert ke UserList
	var userList models.UserList
	err = database.DB.Where("user_id = ? AND movie_id = ?", userID, movie.ID).First(&userList).Error

	if err != nil {
		// Entry baru
		userList = models.UserList{
			UserID:          userID,
			MovieID:         movie.ID,
			Status:          input.Status,
			Rating:          input.Rating,
			Favorite:        input.Favorite,
			Notes:           input.Notes,
			EpisodesWatched: input.EpisodesWatched,
			TotalEpisodes:   input.TotalEpisodes,
		}
		if err := database.DB.Create(&userList).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal menambahkan ke watchlist"})
		}
	} else {
		// Update entry yang sudah ada
		userList.Status = input.Status
		userList.Rating = input.Rating
		userList.Favorite = input.Favorite
		userList.Notes = input.Notes
		userList.EpisodesWatched = input.EpisodesWatched
		if input.TotalEpisodes > 0 {
			userList.TotalEpisodes = input.TotalEpisodes
		}
		database.DB.Save(&userList)
	}

	// Load detail relation
	database.DB.Preload("Movie").First(&userList, userList.ID)

	return c.Status(200).JSON(fiber.Map{
		"message": "Berhasil menyimpan ke watchlist",
		"data":    userList,
	})
}

// GetWatchlist retrieves user's watchlist with optional filters
func GetWatchlist(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	userID := uint(userIDVal.(float64))

	statusFilter := c.Query("status")
	favoriteFilter := c.Query("favorite")
	mediaTypeFilter := c.Query("media_type")

	query := database.DB.Model(&models.UserList{}).
		Preload("Movie").
		Where("user_lists.user_id = ?", userID)

	if statusFilter != "" {
		query = query.Where("user_lists.status = ?", statusFilter)
	}
	if favoriteFilter == "true" {
		query = query.Where("user_lists.favorite = ?", true)
	}

	if mediaTypeFilter != "" {
		query = query.Joins("JOIN movies ON movies.id = user_lists.movie_id").
			Where("movies.media_type = ?", mediaTypeFilter)
	}

	var list []models.UserList
	if err := query.Order("user_lists.updated_at desc").Find(&list).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil watchlist"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    list,
	})
}

// UpdateWatchlistItem updates status, rating, or notes of an item
func UpdateWatchlistItem(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	userID := uint(userIDVal.(float64))

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var userList models.UserList
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&userList).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Item watchlist tidak ditemukan"})
	}

	var input WatchlistInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input JSON tidak valid"})
	}

	if input.Status != "" {
		userList.Status = input.Status
	}
	userList.Rating = input.Rating
	userList.Favorite = input.Favorite
	userList.Notes = input.Notes
	userList.EpisodesWatched = input.EpisodesWatched
	if input.TotalEpisodes > 0 {
		userList.TotalEpisodes = input.TotalEpisodes
	}

	database.DB.Save(&userList)
	database.DB.Preload("Movie").First(&userList, userList.ID)

	return c.JSON(fiber.Map{
		"message": "Watchlist berhasil diperbarui",
		"data":    userList,
	})
}

// DeleteWatchlistItem removes an item from user's list
func DeleteWatchlistItem(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	userID := uint(userIDVal.(float64))

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	result := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.UserList{})
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Item watchlist tidak ditemukan"})
	}

	return c.JSON(fiber.Map{
		"message": "Item berhasil dihapus dari watchlist",
	})
}

// CheckWatchlistItem checks if a TMDB item is already in user's list
func CheckWatchlistItem(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	userID := uint(userIDVal.(float64))

	tmdbID, err := strconv.Atoi(c.Params("tmdbId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "TMDB ID tidak valid"})
	}
	mediaType := c.Query("type", "movie")

	var movie models.Movie
	err = database.DB.Where("tmdb_id = ? AND media_type = ?", tmdbID, mediaType).First(&movie).Error
	if err != nil {
		return c.JSON(fiber.Map{
			"in_watchlist": false,
		})
	}

	var userList models.UserList
	err = database.DB.Preload("Movie").Where("user_id = ? AND movie_id = ?", userID, movie.ID).First(&userList).Error
	if err != nil {
		return c.JSON(fiber.Map{
			"in_watchlist": false,
		})
	}

	return c.JSON(fiber.Map{
		"in_watchlist": true,
		"data":         userList,
	})
}
