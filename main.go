package main

import (
	"log"
	"os"
	"strings"
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
	_ = godotenv.Load(".env")
	_ = godotenv.Load()
	_ = godotenv.Load("api/.env")
	_ = godotenv.Load("../.env")

	database.ConnectDB()

	app := fiber.New(fiber.Config{
		BodyLimit:    10 * 1024 * 1024, // 10MB payload ceiling
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

	// General API rate limiter (600 requests / minute)
	app.Use(limiter.New(limiter.Config{
		Max:        600,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			clientIP := c.Get("CF-Connecting-IP")
			if clientIP == "" {
				clientIP = c.Get("X-Forwarded-For")
			}
			if clientIP == "" {
				clientIP = c.IP()
			}
			return strings.Split(clientIP, ",")[0]
		},
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

	// Register API endpoints with both /api prefix and root prefix
	// This guarantees requests work seamlessly regardless of whether the reverse-proxy strips /api or not.
	registerAppRoutes(app.Group("/api"), authLimiter)
	registerAppRoutes(app, authLimiter)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server running on port %s", port)
	app.Listen(":" + port)
}

func registerAppRoutes(r fiber.Router, authLimiter fiber.Handler) {
	// Auth
	authGroup := r.Group("/auth")
	authGroup.Post("/register", authLimiter, controllers.Register)
	authGroup.Post("/login", authLimiter, controllers.Login)

	// Me (Protected)
	meGroup := r.Group("/me")
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
	meGroup.Get("/feed", controllers.GetSocialFeed)
	meGroup.Post("/follow/:id", controllers.FollowUser)
	meGroup.Delete("/follow/:id", controllers.UnfollowUser)
	meGroup.Get("/notifications", controllers.GetNotifications)
	meGroup.Get("/notifications/unread-count", controllers.GetUnreadNotificationCount)
	meGroup.Put("/notifications/read-all", controllers.MarkAllNotificationsAsRead)
	meGroup.Put("/notifications/:id/read", controllers.MarkNotificationAsRead)
	meGroup.Post("/activity/:id/like", controllers.ToggleActivityLike)
	meGroup.Get("/activity/:id/comments", controllers.GetActivityComments)
	meGroup.Post("/activity/:id/comments", controllers.PostActivityComment)
	meGroup.Delete("/activity/comments/:commentId", controllers.DeleteActivityComment)

	// User legacy & aliases (Protected with explicit middleware per route to avoid prefix collision with /users)
	r.Get("/user/profile", middlewares.Protected(), controllers.GetMe)
	r.Post("/user/avatar", middlewares.Protected(), controllers.UploadAvatar)
	r.Get("/user/watchlist", middlewares.Protected(), controllers.GetWatchlist)
	r.Post("/user/watchlist", middlewares.Protected(), controllers.AddToWatchlist)
	r.Put("/user/watchlist/:id", middlewares.Protected(), controllers.UpdateWatchlistItem)
	r.Put("/user/watchlist/:id/progress", middlewares.Protected(), controllers.IncrementEpisodeProgress)
	r.Put("/user/watchlist/:id/set-progress", middlewares.Protected(), controllers.SetEpisodeWatchedProgress)
	r.Delete("/user/watchlist/:id", middlewares.Protected(), controllers.DeleteWatchlistItem)
	r.Get("/user/watchlist/check/:tmdbId", middlewares.Protected(), controllers.CheckWatchlistItem)

	// Public TMDB
	r.Get("/trending", controllers.GetTrending)
	r.Get("/discover", controllers.GetDiscover)
	r.Get("/search", controllers.SearchMovies)
	r.Get("/detail", controllers.GetMediaDetail)
	r.Get("/tv/season", controllers.GetTVSeasonEpisodes)

	// Public Social & Activity Feeds
	r.Get("/community/activity", controllers.GetSocialFeed)
	r.Get("/community/feed", controllers.GetSocialFeed)
	r.Get("/feed", controllers.GetSocialFeed)
	r.Get("/activity", controllers.GetSocialFeed)
	r.Get("/users/search", controllers.SearchUsers)
	r.Get("/users/discover", controllers.SearchUsers)
	r.Get("/users/:username", controllers.GetPublicProfile)
	r.Get("/users/:username/followers", controllers.GetUserFollowers)
	r.Get("/users/:username/following", controllers.GetUserFollowing)
	r.Get("/users/:username/favorites", controllers.GetPublicFavorites)
	r.Get("/users/:username/ratings", controllers.GetPublicRatings)

	// Direct follow/avatar aliases
	r.Post("/avatar", middlewares.Protected(), controllers.UploadAvatar)
	r.Post("/upload/avatar", middlewares.Protected(), controllers.UploadAvatar)
	r.Post("/users/:id/follow", middlewares.Protected(), controllers.FollowUser)
	r.Delete("/users/:id/follow", middlewares.Protected(), controllers.UnfollowUser)
	r.Post("/users/follow/:id", middlewares.Protected(), controllers.FollowUser)
	r.Delete("/users/follow/:id", middlewares.Protected(), controllers.UnfollowUser)

	// Activity & Notification aliases
	r.Get("/activity/:id/comments", controllers.GetActivityComments)
	r.Post("/activity/:id/like", middlewares.Protected(), controllers.ToggleActivityLike)
	r.Post("/activity/:id/comments", middlewares.Protected(), controllers.PostActivityComment)
	r.Delete("/activity/comments/:commentId", middlewares.Protected(), controllers.DeleteActivityComment)
	r.Get("/notifications", middlewares.Protected(), controllers.GetNotifications)
	r.Get("/notifications/unread-count", middlewares.Protected(), controllers.GetUnreadNotificationCount)
	r.Put("/notifications/read-all", middlewares.Protected(), controllers.MarkAllNotificationsAsRead)
	r.Put("/notifications/:id/read", middlewares.Protected(), controllers.MarkNotificationAsRead)
}
