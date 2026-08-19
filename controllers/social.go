package controllers

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	"cinelog-api/database"
	"cinelog-api/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func getOptionalUserID(c *fiber.Ctx) uint {
	userIDVal := c.Locals("user_id")
	if userIDVal != nil {
		return uint(userIDVal.(float64))
	}

	authHeader := c.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "cinelog_default_secure_secret_key_123"
		}
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err == nil && token.Valid {
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if uid, ok := claims["user_id"].(float64); ok {
					return uint(uid)
				}
			}
		}
	}
	return 0
}

func publicUserResponse(user models.User) fiber.Map {
	return fiber.Map{
		"id":         user.ID,
		"username":   user.Username,
		"bio":        user.Bio,
		"avatar_url": user.AvatarURL,
		"is_public":  user.IsPublic,
		"created_at": user.CreatedAt,
	}
}

func publicLibraryEntry(entry models.UserList) fiber.Map {
	data := fiber.Map{
		"id":         entry.ID,
		"status":     entry.Status,
		"created_at": entry.CreatedAt,
		"updated_at": entry.UpdatedAt,
		"movie":      entry.Movie,
	}

	if entry.VisibilityRating != "private" && entry.Rating > 0 {
		data["rating"] = entry.Rating
	}
	if entry.VisibilityFavorite != "private" {
		data["favorite"] = entry.Favorite
	}

	return data
}

// findUserByIdentifier locates a user by ID or Username (handling URL-encoding, spaces, plus signs, and case-insensitivity)
func findUserByIdentifier(identifier string) (*models.User, error) {
	raw := strings.TrimSpace(identifier)
	if raw == "" {
		return nil, gorm.ErrRecordNotFound
	}

	decoded, err := url.PathUnescape(raw)
	if err != nil || decoded == "" {
		decoded, _ = url.QueryUnescape(raw)
	}
	if decoded == "" {
		decoded = raw
	}
	queryDecoded, _ := url.QueryUnescape(raw)
	if queryDecoded == "" {
		queryDecoded = decoded
	}

	var user models.User
	db := database.DB.Select("id, username, bio, avatar_url, is_public, created_at")

	// 1. Try numeric ID if identifier parses as integer
	if uid, err := strconv.ParseUint(decoded, 10, 64); err == nil && uid > 0 {
		if err := db.Where("id = ?", uint(uid)).First(&user).Error; err == nil {
			return &user, nil
		}
	}

	// 2. Try match username case-insensitively across decoded, raw, and space-replaced variants
	candidateMap := make(map[string]struct{})
	for _, cand := range []string{
		decoded,
		queryDecoded,
		raw,
		strings.ReplaceAll(raw, "+", " "),
		strings.ReplaceAll(raw, "%20", " "),
		strings.TrimSpace(decoded),
		strings.TrimSpace(queryDecoded),
	} {
		trimmed := strings.ToLower(strings.TrimSpace(cand))
		if trimmed != "" {
			candidateMap[trimmed] = struct{}{}
		}
	}

	candidates := make([]string, 0, len(candidateMap))
	for c := range candidateMap {
		candidates = append(candidates, c)
	}

	if err := db.Where("LOWER(username) IN (?)", candidates).First(&user).Error; err == nil {
		return &user, nil
	}

	return nil, gorm.ErrRecordNotFound
}

// SearchUsers discovers and searches community users
func SearchUsers(c *fiber.Ctx) error {
	query := strings.TrimSpace(c.Query("q"))
	currentUserID := getOptionalUserID(c)

	dbQuery := database.DB.Select("id, username, bio, avatar_url, created_at, is_public").Where("is_public = ? OR is_public IS NULL", true)
	if query != "" {
		dbQuery = dbQuery.Where("LOWER(username) LIKE ? OR LOWER(bio) LIKE ?", "%"+strings.ToLower(query)+"%", "%"+strings.ToLower(query)+"%")
	}

	var users []models.User
	if err := dbQuery.Order("created_at desc").Limit(60).Find(&users).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to search users"})
	}

	var results []fiber.Map
	for _, u := range users {
		var followersCount int64
		database.DB.Model(&models.UserFollow{}).Where("following_user_id = ?", u.ID).Count(&followersCount)

		var followingCount int64
		database.DB.Model(&models.UserFollow{}).Where("follower_user_id = ?", u.ID).Count(&followingCount)

		var watchedCount int64
		database.DB.Model(&models.UserList{}).Where("user_id = ?", u.ID).Count(&watchedCount)

		var isFollowing bool
		if currentUserID > 0 && currentUserID != u.ID {
			var follow models.UserFollow
			if err := database.DB.Where("follower_user_id = ? AND following_user_id = ?", currentUserID, u.ID).First(&follow).Error; err == nil {
				isFollowing = true
			}
		}

		results = append(results, fiber.Map{
			"id":              u.ID,
			"username":        u.Username,
			"bio":             u.Bio,
			"avatar_url":      u.AvatarURL,
			"followers_count": followersCount,
			"following_count": followingCount,
			"watched_count":   watchedCount,
			"is_following":    isFollowing,
			"is_self":         currentUserID == u.ID,
			"created_at":      u.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    results,
	})
}

func GetPublicProfile(c *fiber.Ctx) error {
	rawParam := strings.TrimSpace(c.Params("username"))
	if rawParam == "" || strings.EqualFold(rawParam, "search") || strings.EqualFold(rawParam, "discover") {
		return SearchUsers(c)
	}

	user, err := findUserByIdentifier(rawParam)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	currentUserID := getOptionalUserID(c)
	if !user.IsPublic && (currentUserID == 0 || currentUserID != user.ID) {
		return c.Status(403).JSON(fiber.Map{"error": "This user profile is private"})
	}

	var favoriteCount int64
	database.DB.Model(&models.UserList{}).Where("user_id = ? AND favorite = ? AND visibility_favorite <> ?", user.ID, true, "private").Count(&favoriteCount)

	var ratingCount int64
	database.DB.Model(&models.UserList{}).Where("user_id = ? AND rating > 0 AND visibility_rating <> ?", user.ID, "private").Count(&ratingCount)

	var followersCount int64
	database.DB.Model(&models.UserFollow{}).Where("following_user_id = ?", user.ID).Count(&followersCount)

	var followingCount int64
	database.DB.Model(&models.UserFollow{}).Where("follower_user_id = ?", user.ID).Count(&followingCount)

	var isFollowing bool
	if currentUserID > 0 && currentUserID != user.ID {
		var follow models.UserFollow
		if err := database.DB.Where("follower_user_id = ? AND following_user_id = ?", currentUserID, user.ID).First(&follow).Error; err == nil {
			isFollowing = true
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"user":            publicUserResponse(*user),
			"favorite_count":  favoriteCount,
			"rating_count":    ratingCount,
			"followers_count": followersCount,
			"following_count": followingCount,
			"is_following":    isFollowing,
			"is_self":         currentUserID == user.ID,
		},
	})
}

func FollowUser(c *fiber.Ctx) error {
	followerID, err := GetContextUserID(c)
	if err != nil {
		return err
	}

	targetParam := strings.TrimSpace(c.Params("id"))
	if targetParam == "" {
		targetParam = strings.TrimSpace(c.Params("username"))
	}

	target, err := findUserByIdentifier(targetParam)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Target user not found"})
	}

	if followerID == target.ID {
		return c.Status(400).JSON(fiber.Map{"error": "You cannot follow yourself"})
	}

	follow := models.UserFollow{FollowerUserID: followerID, FollowingUserID: target.ID}
	database.DB.Where(follow).FirstOrCreate(&follow)

	return c.JSON(fiber.Map{"success": true, "message": "User followed successfully", "data": follow})
}

func UnfollowUser(c *fiber.Ctx) error {
	followerID, err := GetContextUserID(c)
	if err != nil {
		return err
	}

	targetParam := strings.TrimSpace(c.Params("id"))
	if targetParam == "" {
		targetParam = strings.TrimSpace(c.Params("username"))
	}

	target, err := findUserByIdentifier(targetParam)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Target user not found"})
	}

	database.DB.Where("follower_user_id = ? AND following_user_id = ?", followerID, target.ID).Delete(&models.UserFollow{})
	return c.JSON(fiber.Map{"success": true, "message": "User unfollowed successfully"})
}

func GetUserFollowers(c *fiber.Ctx) error {
	rawParam := strings.TrimSpace(c.Params("username"))
	user, err := findUserByIdentifier(rawParam)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	var follows []models.UserFollow
	if err := database.DB.Preload("Follower").Where("following_user_id = ?", user.ID).Order("created_at desc").Find(&follows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch followers"})
	}

	currentUserID := getOptionalUserID(c)
	results := make([]fiber.Map, 0, len(follows))
	for _, f := range follows {
		if f.Follower.ID == 0 {
			continue
		}
		var isFollowing bool
		if currentUserID > 0 && currentUserID != f.Follower.ID {
			var check models.UserFollow
			if err := database.DB.Where("follower_user_id = ? AND following_user_id = ?", currentUserID, f.Follower.ID).First(&check).Error; err == nil {
				isFollowing = true
			}
		}
		results = append(results, fiber.Map{
			"id":           f.Follower.ID,
			"username":     f.Follower.Username,
			"bio":          f.Follower.Bio,
			"avatar_url":   f.Follower.AvatarURL,
			"is_following": isFollowing,
			"is_self":      currentUserID == f.Follower.ID,
		})
	}

	return c.JSON(fiber.Map{"success": true, "data": results})
}

func GetUserFollowing(c *fiber.Ctx) error {
	rawParam := strings.TrimSpace(c.Params("username"))
	user, err := findUserByIdentifier(rawParam)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	var follows []models.UserFollow
	if err := database.DB.Preload("Following").Where("follower_user_id = ?", user.ID).Order("created_at desc").Find(&follows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch following list"})
	}

	currentUserID := getOptionalUserID(c)
	results := make([]fiber.Map, 0, len(follows))
	for _, f := range follows {
		if f.Following.ID == 0 {
			continue
		}
		var isFollowing bool
		if currentUserID > 0 && currentUserID != f.Following.ID {
			var check models.UserFollow
			if err := database.DB.Where("follower_user_id = ? AND following_user_id = ?", currentUserID, f.Following.ID).First(&check).Error; err == nil {
				isFollowing = true
			}
		}
		results = append(results, fiber.Map{
			"id":           f.Following.ID,
			"username":     f.Following.Username,
			"bio":          f.Following.Bio,
			"avatar_url":   f.Following.AvatarURL,
			"is_following": isFollowing,
			"is_self":      currentUserID == f.Following.ID,
		})
	}

	return c.JSON(fiber.Map{"success": true, "data": results})
}

func GetPublicFavorites(c *fiber.Ctx) error {
	return getPublicEntries(c, true)
}

func GetPublicRatings(c *fiber.Ctx) error {
	return getPublicEntries(c, false)
}

func getPublicEntries(c *fiber.Ctx, favoritesOnly bool) error {
	rawParam := strings.TrimSpace(c.Params("username"))
	user, err := findUserByIdentifier(rawParam)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	currentUserID := getOptionalUserID(c)
	if !user.IsPublic && (currentUserID == 0 || currentUserID != user.ID) {
		return c.Status(403).JSON(fiber.Map{"error": "This user profile is private"})
	}

	query := database.DB.Preload("Movie").Where("user_id = ?", user.ID)
	if favoritesOnly {
		query = query.Where("favorite = ? AND (visibility_favorite <> ? OR visibility_favorite IS NULL)", true, "private")
	} else {
		query = query.Where("rating > 0 AND (visibility_rating <> ? OR visibility_rating IS NULL)", "private")
	}

	var entries []models.UserList
	if err := query.Order("updated_at desc").Find(&entries).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch public data"})
	}

	data := make([]fiber.Map, 0, len(entries))
	for _, entry := range entries {
		data = append(data, publicLibraryEntry(entry))
	}

	return c.JSON(fiber.Map{"success": true, "data": data})
}

