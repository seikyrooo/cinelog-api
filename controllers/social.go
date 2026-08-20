package controllers

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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

// GetSocialFeed returns recent media watch, review, and rating activity feed from followed users (and self) or the community
func GetSocialFeed(c *fiber.Ctx) error {
	currentUserID := getOptionalUserID(c)
	limit, _ := strconv.Atoi(c.Query("limit", "30"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	feedType := c.Query("type", "following")

	var followingIDs []uint
	if currentUserID > 0 {
		database.DB.Model(&models.UserFollow{}).
			Where("follower_user_id = ?", currentUserID).
			Pluck("following_user_id", &followingIDs)
	}

	query := database.DB.Model(&models.UserList{}).
		Preload("User").
		Preload("Movie").
		Joins("JOIN users ON users.id = user_lists.user_id").
		Where("users.is_public = true AND user_lists.rating > 0 AND (user_lists.is_public_feed = true OR user_lists.is_public_feed IS NULL)")

	// If filtering by following: include accounts the user follows PLUS their own activity!
	if feedType == "following" && currentUserID > 0 {
		feedUserIDs := append(followingIDs, currentUserID)
		query = query.Where("user_lists.user_id IN (?)", feedUserIDs)
	} else if feedType == "following" && len(followingIDs) > 0 {
		query = query.Where("user_lists.user_id IN (?)", followingIDs)
	}

	var entries []models.UserList
	if err := query.Order("user_lists.updated_at desc").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch activity feed"})
	}

	// Batch gather entry IDs for like and comment aggregations
	entryIDs := make([]uint, 0, len(entries))
	for _, e := range entries {
		entryIDs = append(entryIDs, e.ID)
	}

	// Map of likes count
	likesCountMap := make(map[uint]int64)
	userLikedMap := make(map[uint]bool)
	commentsCountMap := make(map[uint]int64)

	if len(entryIDs) > 0 {
		type CountResult struct {
			UserMediaEntryID uint
			Total            int64
		}
		var likeCounts []CountResult
		database.DB.Model(&models.ActivityLike{}).
			Select("user_media_entry_id, count(*) as total").
			Where("user_media_entry_id IN (?)", entryIDs).
			Group("user_media_entry_id").
			Scan(&likeCounts)
		for _, lc := range likeCounts {
			likesCountMap[lc.UserMediaEntryID] = lc.Total
		}

		if currentUserID > 0 {
			var userLikes []uint
			database.DB.Model(&models.ActivityLike{}).
				Where("user_id = ? AND user_media_entry_id IN (?)", currentUserID, entryIDs).
				Pluck("user_media_entry_id", &userLikes)
			for _, id := range userLikes {
				userLikedMap[id] = true
			}
		}

		var commentCounts []CountResult
		database.DB.Model(&models.ActivityComment{}).
			Select("user_media_entry_id, count(*) as total").
			Where("user_media_entry_id IN (?)", entryIDs).
			Group("user_media_entry_id").
			Scan(&commentCounts)
		for _, cc := range commentCounts {
			commentsCountMap[cc.UserMediaEntryID] = cc.Total
		}
	}

	activities := make([]fiber.Map, 0, len(entries))
	for _, entry := range entries {
		rev := entry.Review
		if rev == "" {
			rev = entry.Notes
		}
		notes := entry.Notes
		if notes == "" {
			notes = entry.Review
		}

		isEdited := entry.UpdatedAt.Sub(entry.CreatedAt) > 1*time.Minute

		activities = append(activities, fiber.Map{
			"id":               entry.ID,
			"user_id":          entry.UserID,
			"status":           entry.Status,
			"rating":           entry.Rating,
			"review":           rev,
			"notes":            notes,
			"favorite":         entry.Favorite,
			"is_public_feed":   entry.IsPublicFeed,
			"season_watched":   entry.SeasonWatched,
			"episodes_watched": entry.EpisodesWatched,
			"created_at":       entry.CreatedAt,
			"updated_at":       entry.UpdatedAt,
			"is_edited":        isEdited,
			"user":             publicUserResponse(entry.User),
			"movie":            entry.Movie,
			"likes_count":      likesCountMap[entry.ID],
			"is_liked":         userLikedMap[entry.ID],
			"comments_count":   commentsCountMap[entry.ID],
		})
	}

	return c.JSON(fiber.Map{
		"success":           true,
		"data":              activities,
		"page":              page,
		"limit":             limit,
		"count":             len(activities),
		"is_following_feed": feedType == "following",
		"following_count":   len(followingIDs),
	})
}

// ToggleActivityLike likes or unlikes an activity entry
func ToggleActivityLike(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	activityID, err := strconv.Atoi(c.Params("id"))
	if err != nil || activityID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid activity ID"})
	}

	var entry models.UserList
	if err := database.DB.Preload("Movie").First(&entry, activityID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Activity entry not found"})
	}

	var existingLike models.ActivityLike
	findErr := database.DB.Where("user_id = ? AND user_media_entry_id = ?", userID, activityID).First(&existingLike).Error

	var liked bool
	if findErr == nil {
		// Unlike
		database.DB.Delete(&existingLike)
		liked = false
	} else {
		// Like
		newLike := models.ActivityLike{
			UserID:           userID,
			UserMediaEntryID: uint(activityID),
		}
		database.DB.Create(&newLike)
		liked = true

		// Trigger notification to the entry owner if not self
		if entry.UserID != userID {
			var actor models.User
			database.DB.First(&actor, userID)

			movieTitle := entry.Movie.Title
			if movieTitle == "" {
				movieTitle = "movie / series"
			}
			msg := fmt.Sprintf("@%s liked your log on %s", actor.Username, movieTitle)

			notif := models.Notification{
				RecipientUserID:  entry.UserID,
				ActorUserID:      userID,
				UserMediaEntryID: entry.ID,
				Type:             "like",
				Message:          msg,
				IsRead:           false,
			}
			database.DB.Create(&notif)
		}
	}

	var likesCount int64
	database.DB.Model(&models.ActivityLike{}).Where("user_media_entry_id = ?", activityID).Count(&likesCount)

	return c.JSON(fiber.Map{
		"success":     true,
		"liked":       liked,
		"likes_count": likesCount,
	})
}

// GetActivityComments retrieves all comments for an activity entry
func GetActivityComments(c *fiber.Ctx) error {
	activityID, err := strconv.Atoi(c.Params("id"))
	if err != nil || activityID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid activity ID"})
	}

	var comments []models.ActivityComment
	if err := database.DB.Preload("User").
		Where("user_media_entry_id = ?", activityID).
		Order("created_at asc").
		Find(&comments).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch comments"})
	}

	data := make([]fiber.Map, 0, len(comments))
	for _, comm := range comments {
		data = append(data, fiber.Map{
			"id":         comm.ID,
			"user_id":    comm.UserID,
			"content":    comm.Content,
			"created_at": comm.CreatedAt,
			"updated_at": comm.UpdatedAt,
			"user":       publicUserResponse(comm.User),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    data,
		"count":   len(data),
	})
}

// PostActivityComment adds a new comment to an activity entry
func PostActivityComment(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	activityID, err := strconv.Atoi(c.Params("id"))
	if err != nil || activityID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid activity ID"})
	}

	type CommentInput struct {
		Content string `json:"content"`
	}
	var input CommentInput
	if err := c.BodyParser(&input); err != nil || strings.TrimSpace(input.Content) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Comment content cannot be empty"})
	}

	var entry models.UserList
	if err := database.DB.Preload("Movie").First(&entry, activityID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Activity entry not found"})
	}

	comment := models.ActivityComment{
		UserID:           userID,
		UserMediaEntryID: uint(activityID),
		Content:          strings.TrimSpace(input.Content),
	}
	if err := database.DB.Create(&comment).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save comment"})
	}

	// Preload author user
	database.DB.Preload("User").First(&comment, comment.ID)

	// Trigger notification to the entry owner if not self
	if entry.UserID != userID {
		snippet := input.Content
		if len(snippet) > 40 {
			snippet = snippet[:40] + "..."
		}
		msg := fmt.Sprintf("@%s commented on your log: \"%s\"", comment.User.Username, snippet)

		notif := models.Notification{
			RecipientUserID:  entry.UserID,
			ActorUserID:      userID,
			UserMediaEntryID: entry.ID,
			Type:             "comment",
			Message:          msg,
			IsRead:           false,
		}
		database.DB.Create(&notif)
	}

	var commentsCount int64
	database.DB.Model(&models.ActivityComment{}).Where("user_media_entry_id = ?", activityID).Count(&commentsCount)

	return c.Status(201).JSON(fiber.Map{
		"success":        true,
		"data":           comment,
		"comments_count": commentsCount,
	})
}

// DeleteActivityComment removes a comment by its author
func DeleteActivityComment(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	commentID, err := strconv.Atoi(c.Params("commentId"))
	if err != nil || commentID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid comment ID"})
	}

	var comment models.ActivityComment
	if err := database.DB.Where("id = ? AND user_id = ?", commentID, userID).First(&comment).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Comment not found or permission denied"})
	}

	database.DB.Delete(&comment)
	return c.JSON(fiber.Map{"success": true, "message": "Comment deleted"})
}

// GetNotifications returns user's notifications with unread count
func GetNotifications(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var notifications []models.Notification
	if err := database.DB.Preload("Actor").
		Where("recipient_user_id = ?", userID).
		Order("created_at desc").
		Limit(30).
		Find(&notifications).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch notifications"})
	}

	var unreadCount int64
	database.DB.Model(&models.Notification{}).
		Where("recipient_user_id = ? AND is_read = false", userID).
		Count(&unreadCount)

	data := make([]fiber.Map, 0, len(notifications))
	for _, n := range notifications {
		data = append(data, fiber.Map{
			"id":                  n.ID,
			"type":                n.Type,
			"message":             n.Message,
			"is_read":             n.IsRead,
			"user_media_entry_id": n.UserMediaEntryID,
			"created_at":          n.CreatedAt,
			"actor":               publicUserResponse(n.Actor),
		})
	}

	return c.JSON(fiber.Map{
		"success":      true,
		"data":         data,
		"unread_count": unreadCount,
	})
}

// GetUnreadNotificationCount returns just the count for navbar badges
func GetUnreadNotificationCount(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var unreadCount int64
	database.DB.Model(&models.Notification{}).
		Where("recipient_user_id = ? AND is_read = false", userID).
		Count(&unreadCount)

	return c.JSON(fiber.Map{
		"success":      true,
		"unread_count": unreadCount,
	})
}

// MarkNotificationAsRead marks a single notification as read
func MarkNotificationAsRead(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	notifID, err := strconv.Atoi(c.Params("id"))
	if err != nil || notifID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid notification ID"})
	}

	database.DB.Model(&models.Notification{}).
		Where("id = ? AND recipient_user_id = ?", notifID, userID).
		Update("is_read", true)

	return c.JSON(fiber.Map{"success": true})
}

// MarkAllNotificationsAsRead marks all user notifications as read
func MarkAllNotificationsAsRead(c *fiber.Ctx) error {
	userID, err := GetContextUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	database.DB.Model(&models.Notification{}).
		Where("recipient_user_id = ? AND is_read = false", userID).
		Update("is_read", true)

	return c.JSON(fiber.Map{"success": true, "message": "All notifications marked as read"})
}


