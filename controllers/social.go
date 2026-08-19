package controllers

import (
	"os"
	"strconv"
	"strings"

	"cinelog-api/database"
	"cinelog-api/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
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

// SearchUsers discovers and searches community users
func SearchUsers(c *fiber.Ctx) error {
	query := strings.TrimSpace(c.Query("q"))
	currentUserID := getOptionalUserID(c)

	dbQuery := database.DB.Select("id, username, bio, avatar_url, created_at, is_public").Where("is_public = ?", true)
	if query != "" {
		dbQuery = dbQuery.Where("LOWER(username) LIKE ? OR LOWER(bio) LIKE ?", "%"+strings.ToLower(query)+"%", "%"+strings.ToLower(query)+"%")
	}

	var users []models.User
	if err := dbQuery.Order("created_at desc").Limit(40).Find(&users).Error; err != nil {
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
	username := c.Params("username")
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Username parameter is required"})
	}

	var user models.User
	if err := database.DB.Select("id, username, bio, avatar_url, is_public, created_at").Where("username = ?", username).First(&user).Error; err != nil {
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
			"user":            publicUserResponse(user),
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

	followingID64, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid user ID"})
	}
	followingID := uint(followingID64)
	if followerID == followingID {
		return c.Status(400).JSON(fiber.Map{"error": "You cannot follow yourself"})
	}

	var target models.User
	if err := database.DB.First(&target, followingID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Target user not found"})
	}

	follow := models.UserFollow{FollowerUserID: followerID, FollowingUserID: followingID}
	database.DB.Where(follow).FirstOrCreate(&follow)

	return c.JSON(fiber.Map{"message": "User followed successfully", "data": follow})
}

func UnfollowUser(c *fiber.Ctx) error {
	followerID, err := GetContextUserID(c)
	if err != nil {
		return err
	}

	followingID64, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	database.DB.Where("follower_user_id = ? AND following_user_id = ?", followerID, uint(followingID64)).Delete(&models.UserFollow{})
	return c.JSON(fiber.Map{"message": "User unfollowed successfully"})
}

func GetUserFollowers(c *fiber.Ctx) error {
	username := c.Params("username")
	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	var follows []models.UserFollow
	if err := database.DB.Preload("Follower").Where("following_user_id = ?", user.ID).Find(&follows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch followers"})
	}

	currentUserID := getOptionalUserID(c)
	var results []fiber.Map
	for _, f := range follows {
		if !f.Follower.IsPublic && f.Follower.ID != currentUserID {
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
	username := c.Params("username")
	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	var follows []models.UserFollow
	if err := database.DB.Preload("Following").Where("follower_user_id = ?", user.ID).Find(&follows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch following list"})
	}

	currentUserID := getOptionalUserID(c)
	var results []fiber.Map
	for _, f := range follows {
		if !f.Following.IsPublic && f.Following.ID != currentUserID {
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
	username := c.Params("username")
	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}
	currentUserID := getOptionalUserID(c)
	if !user.IsPublic && (currentUserID == 0 || currentUserID != user.ID) {
		return c.Status(403).JSON(fiber.Map{"error": "This user profile is private"})
	}

	query := database.DB.Preload("Movie").Where("user_id = ?", user.ID)
	if favoritesOnly {
		query = query.Where("favorite = ? AND visibility_favorite <> ?", true, "private")
	} else {
		query = query.Where("rating > 0 AND visibility_rating <> ?", "private")
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

