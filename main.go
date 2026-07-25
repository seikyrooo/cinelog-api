package main

import (
    "log"
    "os"

    "cinelog-api/database"
    "cinelog-api/controllers"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/logger"
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
    app.Get("/api/search", controllers.SearchMovies)

    port := os.Getenv("PORT")
    if port == "" {
        port = "3000"
    }

    log.Printf("Server berjalan di port %s", port)
    app.Listen(":" + port)
}