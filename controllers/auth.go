package controllers

import (
	"os"
	"time"

	"cinelog-api/database"
	"cinelog-api/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Struktur data yang diharapkan dari input user
type AuthInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ProfileInput struct {
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
	IsPublic  *bool  `json:"is_public"`
}

func safeUserResponse(user models.User) fiber.Map {
	return fiber.Map{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"bio":        user.Bio,
		"avatar_url": user.AvatarURL,
		"is_public":  user.IsPublic,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	}
}

// REGISTER USER BARU
func Register(c *fiber.Ctx) error {
	var input AuthInput

	// Parsing JSON dari body request
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}

	// Hash Password menggunakan Bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengenkripsi password"})
	}

	// Buat objek User baru
	user := models.User{
		Username: input.Username,
		Email:    input.Email,
		Password: string(hashedPassword),
		IsPublic: true,
	}

	// Simpan ke Database
	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Username atau Email sudah terdaftar"})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "User berhasil mendaftar!",
		"user_id": user.ID,
		"user":    safeUserResponse(user),
	})
}

// LOGIN USER & GENERATE JWT
func Login(c *fiber.Ctx) error {
	var input AuthInput
	var user models.User

	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}

	// Cari user berdasarkan email
	database.DB.Where("email = ?", input.Email).First(&user)
	if user.ID == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "User tidak ditemukan"})
	}

	// Bandingkan password yang diinput dengan hash di database
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Password salah!"})
	}

	// Buat JWT Token
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = user.ID
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix() // Token kedaluwarsa dalam 3 hari

	// Sign token dengan JWT_SECRET dari .env
	t, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membuat token"})
	}

	return c.JSON(fiber.Map{
		"message": "Login berhasil!",
		"token":   t,
		"user_id": user.ID,
		"user":    safeUserResponse(user),
	})
}

func GetMe(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var user models.User
	if err := database.DB.First(&user, uint(userIDVal.(float64))).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User tidak ditemukan"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    safeUserResponse(user),
	})
}

func PatchMe(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var input ProfileInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input JSON tidak valid"})
	}

	var user models.User
	if err := database.DB.First(&user, uint(userIDVal.(float64))).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User tidak ditemukan"})
	}

	user.Bio = input.Bio
	user.AvatarURL = input.AvatarURL
	if input.IsPublic != nil {
		user.IsPublic = *input.IsPublic
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal menyimpan profile"})
	}

	return c.JSON(fiber.Map{
		"message": "Profile berhasil diperbarui",
		"data":    safeUserResponse(user),
	})
}
