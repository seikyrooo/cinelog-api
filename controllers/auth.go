package controllers

import (
	"os"
	"strings"
	"time"

	"cinelog-api/database"
	"cinelog-api/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

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

// Register creates a new user
func Register(c *fiber.Ctx) error {
	var input AuthInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	username := strings.TrimSpace(input.Username)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := input.Password

	if username == "" || email == "" || password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Username, email, and password are required"})
	}

	if len(username) < 3 {
		return c.Status(400).JSON(fiber.Map{"error": "Username must be at least 3 characters long"})
	}

	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return c.Status(400).JSON(fiber.Map{"error": "Please provide a valid email address"})
	}

	if len(password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "Password must be at least 6 characters long"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt password"})
	}

	user := models.User{
		Username: username,
		Email:    email,
		Password: string(hashedPassword),
		IsPublic: true,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Username or email is already registered"})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "User registered successfully",
		"user_id": user.ID,
		"user":    safeUserResponse(user),
	})
}

// Login authenticates user and returns JWT token
func Login(c *fiber.Ctx) error {
	var input AuthInput
	var user models.User

	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := input.Password

	if email == "" || password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Email and password are required"})
	}

	database.DB.Where("LOWER(email) = ?", email).First(&user)
	if user.ID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid email or password"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid email or password"})
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "cinelog_default_secure_secret_key_123"
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = user.ID
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix() // 3 days expiration

	t, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate authentication token"})
	}

	return c.JSON(fiber.Map{
		"message": "Login successful",
		"token":   t,
		"user_id": user.ID,
		"user":    safeUserResponse(user),
	})
}

func GetMe(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized access"})
	}

	var user models.User
	if err := database.DB.First(&user, uint(userIDVal.(float64))).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    safeUserResponse(user),
	})
}

func PatchMe(c *fiber.Ctx) error {
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized access"})
	}

	var input ProfileInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON input"})
	}

	var user models.User
	if err := database.DB.First(&user, uint(userIDVal.(float64))).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	user.Bio = strings.TrimSpace(input.Bio)
	user.AvatarURL = strings.TrimSpace(input.AvatarURL)
	if input.IsPublic != nil {
		user.IsPublic = *input.IsPublic
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update profile"})
	}

	return c.JSON(fiber.Map{
		"message": "Profile updated successfully",
		"data":    safeUserResponse(user),
	})
}

