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
	}

	// Simpan ke Database
	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Username atau Email sudah terdaftar"})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "User berhasil mendaftar!",
		"user_id": user.ID,
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
	})
}