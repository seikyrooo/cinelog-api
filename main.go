package main

import (
    "log"
    "os"
    "time"

    "cinelog-api/database"
    "cinelog-api/controllers"
    "cinelog-api/middlewares"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/logger"
    "github.com/gofiber/fiber/v2/middleware/limiter"
    "github.com/joho/godotenv"
)

func main() {
    // Load file .env
    if err := godotenv.Load(); err != nil {
        log.Println("Warning: Error loading .env file")
    }

    // Konek ke DB dan migrasi
    database.ConnectDB()

    app := fiber.New()
    app.Use(logger.New())

    app.Use(limiter.New(limiter.Config{
        Max:        60,
        Expiration: 1 * time.Minute,
        LimitReached: func(c *fiber.Ctx) error {
            return c.Status(429).JSON(fiber.Map{
                "error": "Terlalu banyak request, mohon tunggu sebentar.",
            })
        },
    }))

    // Route Health Check
    app.Get("/", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{
            "app":    "Cinelog API",
            "status": "Running smoothly!",
        })
    })
    // Grup Route Authentication
    authGroup := app.Group("/api/auth")
    authGroup.Post("/register", controllers.Register)
    authGroup.Post("/login", controllers.Login)

    // --- GRUP ROUTE YANG DILINDUNGI (PRIVATE) ---
	privateGroup := app.Group("/api/user")
    
    // Pasang Middleware "Satpam" ke dalam grup ini
	privateGroup.Use(middlewares.Protected())

	// Endpoint Test: Hanya bisa dibuka jika bawa Token valid
	privateGroup.Get("/profile", func(c *fiber.Ctx) error {
		// Ambil user_id yang tadi disimpan oleh middleware
		userID := c.Locals("user_id")

		return c.JSON(fiber.Map{
			"message": "Selamat datang di area rahasia!",
			"user_id": userID,
		})
	})

    // Route Search Movies
    app.Get("/api/search", controllers.SearchMovies)

    port := os.Getenv("PORT")
    if port == "" {
        port = "3000"
    }

    log.Printf("Server berjalan di port %s", port)
    app.Listen(":" + port)
}