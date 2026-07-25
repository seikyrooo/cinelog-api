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
    // Ambil query parameter "?q=..." dari URL
    query := c.Query("q")
    if query == "" {
        return c.Status(400).JSON(fiber.Map{"error": "Parameter pencarian 'q' wajib diisi"})
    }

    apiKey := os.Getenv("TMDB_API_KEY")
    if apiKey == "" {
        return c.Status(500).JSON(fiber.Map{"error": "TMDB_API_KEY belum dikonfigurasi"})
    }

    // Encode karakter URL (misal spasi menjadi %20)
    escapedQuery := url.QueryEscape(query)
    tmdbURL := "https://api.themoviedb.org/3/search/movie?query=" + escapedQuery + "&api_key=" + apiKey

    // Lakukan HTTP GET Request ke TMDB
    resp, err := http.Get(tmdbURL)
    if err != nil || resp.StatusCode != 200 {
        return c.Status(502).JSON(fiber.Map{"error": "Gagal mengambil data dari TMDB"})
    }
    defer resp.Body.Close()

    // Decode JSON dari TMDB ke dalam struct Go kita
    var tmdbResp models.TMDBResponse
    if err := json.NewDecoder(resp.Body).Decode(&tmdbResp); err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Gagal membaca respons dari TMDB"})
    }

    // Kembalikan hasilnya ke Frontend
    return c.JSON(fiber.Map{
        "success": true,
        "data":    tmdbResp.Results,
    })
}