package middlewares

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Protected adalah satpam yang akan mengecek token
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Ambil header Authorization dari request
		authHeader := c.Get("Authorization")
		
		// 2. Cek apakah formatnya benar (harus dimulai dengan "Bearer ")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Akses ditolak. Token tidak ditemukan atau format salah",
			})
		}

		// 3. Potong tulisan "Bearer " untuk mengambil token aslinya saja
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 4. Validasi Token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Pastikan algoritma enkripsinya sesuai
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "Metode signing tidak valid")
			}
			secret := os.Getenv("JWT_SECRET")
			if secret == "" {
				secret = "cinelog_default_secure_secret_key_123"
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Token tidak valid atau sudah kedaluwarsa",
			})
		}

		// 5. Ekstrak data (claims) dari dalam token
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			// Simpan user_id ke dalam context lokal agar bisa dibaca oleh controller
			c.Locals("user_id", claims["user_id"])
			
			// Lanjutkan ke controller / endpoint yang dituju
			return c.Next()
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
}