package controllers

import (
	"strconv"

	"cinelog-api/database"
	"cinelog-api/models"

	"github.com/gofiber/fiber/v2"
)

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

func GetPublicProfile(c *fiber.Ctx) error {
	username := c.Params("username")
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Username parameter is required"})
	}

	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}
	if !user.IsPublic {
		return c.Status(403).JSON(fiber.Map{"error": "This user profile is private"})
	}

	var favoriteCount int64
	database.DB.Model(&models.UserList{}).Where("user_id = ? AND favorite = ? AND visibility_favorite <> ?", user.ID, true, "private").Count(&favoriteCount)

	var ratingCount int64
	database.DB.Model(&models.UserList{}).Where("user_id = ? AND rating > 0 AND visibility_rating <> ?", user.ID, "private").Count(&ratingCount)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"user":           publicUserResponse(user),
			"favorite_count": favoriteCount,
			"rating_count":   ratingCount,
		},
	})
}

func FollowUser(c *fiber.Ctx) error {
	followerVal := c.Locals("user_id")
	if followerVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized access"})
	}
	followerID := uint(followerVal.(float64))

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
	followerVal := c.Locals("user_id")
	if followerVal == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized access"})
	}
	followerID := uint(followerVal.(float64))

	followingID64, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	database.DB.Where("follower_user_id = ? AND following_user_id = ?", followerID, uint(followingID64)).Delete(&models.UserFollow{})
	return c.JSON(fiber.Map{"message": "User unfollowed successfully"})
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
	if !user.IsPublic {
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

