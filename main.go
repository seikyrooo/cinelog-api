package main

import (
	"log"
	"os"
	"time"

	"cinelog-api/controllers"
	"cinelog-api/database"
	"cinelog-api/middlewares"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Error loading .env file")
	}

	database.ConnectDB()

	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024, // 10MB payload ceiling
		ServerHeader: "CineLog",
	})

	// Security Headers Middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		return c.Next()
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, HEAD, PUT, DELETE, PATCH, OPTIONS",
	}))

	// Response Compression (Gzip / Deflate / Brotli)
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	app.Use(logger.New())

	// General API rate limiter
	app.Use(limiter.New(limiter.Config{
		Max:        120,
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"error": "Too many requests, please try again in a moment.",
			})
		},
	}))

	// Strict Auth Rate Limiter (Brute-force protection: max 15 attempts / 15 mins)
	authLimiter := limiter.New(limiter.Config{
		Max:        15,
		Expiration: 15 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"error": "Too many authentication attempts. Please try again after 15 minutes.",
			})
		},
	})

	// Ensure uploads directories exist on startup
	_ = os.MkdirAll("./uploads/posters", 0755)
	_ = os.MkdirAll("./uploads/backdrops", 0755)
	_ = os.MkdirAll("./uploads/avatars", 0755)

	// Serve Static Uploaded Images with HTTP 1-year Cache Headers & Compression
	staticConfig := fiber.Static{
		Compress:  true,
		ByteRange: true,
		MaxAge:    31536000, // 1 year browser cache
	}
	app.Static("/uploads", "./uploads", staticConfig)
	app.Static("/api/uploads", "./uploads", staticConfig)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"app":    "Cinelog API",
			"status": "Running smoothly and securely!",
		})
	})

	authGroup := app.Group("/api/auth")
	authGroup.Post("/register", authLimiter, controllers.Register)
	authGroup.Post("/login", authLimiter, controllers.Login)

	meGroup := app.Group("/api/me")
	meGroup.Use(middlewares.Protected())
	meGroup.Get("/", controllers.GetMe)
	meGroup.Patch("/", controllers.PatchMe)
	meGroup.Post("/avatar", controllers.UploadAvatar)
	meGroup.Get("/radar", controllers.GetReleaseRadar)
	meGroup.Post("/radar/sync", controllers.SyncRadarAirDates)
	meGroup.Get("/library", controllers.GetWatchlist)
	meGroup.Post("/library", controllers.AddToWatchlist)
	meGroup.Get("/library/:id", controllers.GetLibraryItem)
	meGroup.Put("/library/:id", controllers.UpdateWatchlistItem)
	meGroup.Patch("/library/:id", controllers.UpdateWatchlistItem)
	meGroup.Delete("/library/:id", controllers.DeleteWatchlistItem)
	meGroup.Put("/library/:id/progress", controllers.IncrementEpisodeProgress)
	meGroup.Put("/library/:id/set-progress", controllers.SetEpisodeWatchedProgress)
	meGroup.Put("/library/:id/tv-progress", controllers.SetTVProgress)
	meGroup.Patch("/library/:id/tv-progress", controllers.SetTVProgress)
	meGroup.Get("/library/check/:tmdbId", controllers.CheckLibraryItem)
	meGroup.Post("/follow/:id", controllers.FollowUser)
	meGroup.Delete("/follow/:id", controllers.UnfollowUser)

	app.Get("/api/trending", controllers.GetTrending)
	app.Get("/api/discover", controllers.GetDiscover)
	app.Get("/api/search", controllers.SearchMovies)
	app.Get("/api/detail", controllers.GetMediaDetail)
	app.Get("/api/tv/season", controllers.GetTVSeasonEpisodes)
	app.Get("/api/users/search", controllers.SearchUsers)
	app.Get("/api/users/discover", controllers.SearchUsers)
	app.Get("/api/users/:username", controllers.GetPublicProfile)
	app.Get("/api/users/:username/followers", controllers.GetUserFollowers)
	app.Get("/api/users/:username/following", controllers.GetUserFollowing)
	app.Get("/api/users/:username/favorites", controllers.GetPublicFavorites)
	app.Get("/api/users/:username/ratings", controllers.GetPublicRatings)

	privateGroup := app.Group("/api/user")
	privateGroup.Use(middlewares.Protected())

	privateGroup.Get("/profile", controllers.GetMe)
	privateGroup.Post("/avatar", controllers.UploadAvatar)

	privateGroup.Get("/watchlist", controllers.GetWatchlist)
	privateGroup.Post("/watchlist", controllers.AddToWatchlist)
	privateGroup.Put("/watchlist/:id", controllers.UpdateWatchlistItem)
	privateGroup.Put("/watchlist/:id/progress", controllers.IncrementEpisodeProgress)
	privateGroup.Put("/watchlist/:id/set-progress", controllers.SetEpisodeWatchedProgress)
	privateGroup.Delete("/watchlist/:id", controllers.DeleteWatchlistItem)
	privateGroup.Get("/watchlist/check/:tmdbId", controllers.CheckWatchlistItem)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server running on port %s", port)
	app.Listen(":" + port)
}
