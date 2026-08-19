package controllers

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
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

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)

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

	if !usernameRegex.MatchString(username) {
		return c.Status(400).JSON(fiber.Map{"error": "Username must be 3-30 characters long and contain only letters, numbers, underscores, or hyphens"})
	}

	if _, err := mail.ParseAddress(email); err != nil || len(email) > 100 {
		return c.Status(400).JSON(fiber.Map{"error": "Please provide a valid email address"})
	}

	if len(password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "Password must be at least 6 characters long"})
	}
	if len(password) > 72 {
		return c.Status(400).JSON(fiber.Map{"error": "Password cannot exceed 72 characters"})
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

// GetContextUserID extracts authenticated user ID from context
func GetContextUserID(c *fiber.Ctx) (uint, error) {
	val := c.Locals("user_id")
	if val == nil {
		return 0, c.Status(401).JSON(fiber.Map{"error": "Unauthorized access"})
	}
	return uint(val.(float64)), nil
}

func GetMe(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return err
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    safeUserResponse(user),
	})
}

func PatchMe(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return err
	}

	var input ProfileInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON input"})
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	cleanBio := strings.TrimSpace(input.Bio)
	if len([]rune(cleanBio)) > 500 {
		return c.Status(400).JSON(fiber.Map{"error": "Bio cannot exceed 500 characters"})
	}
	user.Bio = html.EscapeString(cleanBio)

	cleanAvatarURL := strings.TrimSpace(input.AvatarURL)
	if cleanAvatarURL != "" {
		if !strings.HasPrefix(cleanAvatarURL, "http://") && !strings.HasPrefix(cleanAvatarURL, "https://") && !strings.HasPrefix(cleanAvatarURL, "/uploads/") && !strings.HasPrefix(cleanAvatarURL, "uploads/") {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid avatar URL format"})
		}
		if len(cleanAvatarURL) > 500 {
			return c.Status(400).JSON(fiber.Map{"error": "Avatar URL is too long"})
		}
		user.AvatarURL = cleanAvatarURL
	}

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

// UploadAvatar handles direct file upload for user avatar with magic-byte validation
func UploadAvatar(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return err
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Avatar image file is required"})
	}

	// 5MB max size
	if file.Size > 5*1024*1024 {
		return c.Status(400).JSON(fiber.Map{"error": "Image size exceeds maximum limit of 5MB"})
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" {
		return c.Status(400).JSON(fiber.Map{"error": "Allowed image formats: JPG, PNG, WEBP, GIF"})
	}

	// Inspect actual file content via Magic Bytes to prevent disguised uploads
	src, err := file.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Failed to open uploaded file for verification"})
	}
	buf := make([]byte, 512)
	n, err := src.Read(buf)
	src.Close()
	if err != nil && err != io.EOF {
		return c.Status(400).JSON(fiber.Map{"error": "Failed to inspect file format header"})
	}

	mimeType := http.DetectContentType(buf[:n])
	if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" && mimeType != "image/gif" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid file content: only authentic JPG, PNG, WEBP, and GIF images are permitted"})
	}

	_ = os.MkdirAll("./uploads/avatars", 0755)
	filename := fmt.Sprintf("avatar_%d_%d%s", userID, time.Now().UnixNano(), ext)
	savePath := filepath.Join("./uploads/avatars", filename)

	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save avatar image file"})
	}

	avatarURL := "/uploads/avatars/" + filename

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	user.AvatarURL = avatarURL
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update user avatar"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Avatar uploaded successfully",
		"data":    safeUserResponse(user),
	})
}

