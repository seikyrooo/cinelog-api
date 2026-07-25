package main

import (
	"log"
	"os"
	"time"

	"cinelog-api/controllers"
	"cinelog-api/database"
	"cinelog-api/middlewares"

	"github.com/gofiber/fiber/v2"
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

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, HEAD, PUT, DELETE, PATCH, OPTIONS",
	}))

	app.Use(logger.New())

	app.Use(limiter.New(limiter.Config{
		Max:        120,
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"error": "Terlalu banyak request, mohon tunggu sebentar.",
			})
		},
	}))

	// Ensure uploads directories exist on startup
	_ = os.MkdirAll("./uploads/posters", 0755)
	_ = os.MkdirAll("./uploads/backdrops", 0755)

	// Serve Static Uploaded Images (Posters & Backdrops) locally from VPS
	app.Static("/uploads", "./uploads")

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"app":    "Cinelog API",
			"status": "Running smoothly!",
		})
	})

	authGroup := app.Group("/api/auth")
	authGroup.Post("/register", controllers.Register)
	authGroup.Post("/login", controllers.Login)

	app.Get("/api/search", controllers.SearchMovies)

	privateGroup := app.Group("/api/user")
	privateGroup.Use(middlewares.Protected())

	privateGroup.Get("/profile", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id")
		return c.JSON(fiber.Map{
			"message": "Authenticated profile",
			"user_id": userID,
		})
	})

	privateGroup.Get("/watchlist", controllers.GetWatchlist)
	privateGroup.Post("/watchlist", controllers.AddToWatchlist)
	privateGroup.Put("/watchlist/:id", controllers.UpdateWatchlistItem)
	privateGroup.Delete("/watchlist/:id", controllers.DeleteWatchlistItem)
	privateGroup.Get("/watchlist/check/:tmdbId", controllers.CheckWatchlistItem)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server berjalan di port %s", port)
	app.Listen(":" + port)
}