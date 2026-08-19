package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"cinelog-api/database"
	"cinelog-api/models"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type CleanseStats struct {
	InvalidStatusesFixed    int64
	InvalidRatingsClamped   int64
	NullVisibilitiesFixed   int64
	ProgressSynced          int64
	OrphanUserListsDeleted  int64
	OrphanTVProgressDeleted int64
	OrphanFollowsDeleted    int64
	UnusedMoviesPruned      int64
}

func main() {
	dryRunFlag := flag.Bool("dry-run", true, "Preview changes without applying them to database (safe mode)")
	applyFlag := flag.Bool("apply", false, "Execute and apply cleansing fixes to database")
	pruneMediaFlag := flag.Bool("prune-unused-media", false, "Prune cached media (movies/tv) not added to any user watchlist")
	resetFlag := flag.Bool("reset", false, "DANGER: Reset all tables and data in database")
	confirmResetFlag := flag.String("confirm", "", "Confirmation phrase required for reset ('CONFIRM_RESET')")

	flag.Parse()

	// If -apply is set, turn off dryRun
	if *applyFlag {
		*dryRunFlag = false
	}

	if err := godotenv.Load("../../.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("Note: .env file not found, using environment variables directly.")
		}
	}

	database.ConnectDB()
	db := database.DB

	fmt.Println("=========================================================")
	fmt.Println("             CINELOG DATABASE CLEANSER TOOL              ")
	fmt.Println("=========================================================")

	if *resetFlag {
		handleDatabaseReset(db, *confirmResetFlag)
		return
	}

	if *dryRunFlag {
		fmt.Println("🔍 RUNNING IN DRY-RUN MODE (No database changes will be written)")
		fmt.Println("   To apply these fixes, run with: go run cmd/cleanse/main.go -apply")
	} else {
		fmt.Println("⚡ RUNNING IN APPLY MODE (Fixes will be committed to database)")
	}
	fmt.Println("---------------------------------------------------------")

	runCleansing(db, *dryRunFlag, *pruneMediaFlag)
}

func runCleansing(db *gorm.DB, dryRun bool, pruneUnusedMedia bool) {
	stats := CleanseStats{}

	// Begin transaction if applying changes
	var tx *gorm.DB
	if !dryRun {
		tx = db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				log.Fatalf("Fatal error during cleansing, transaction rolled back: %v", r)
			}
		}()
	} else {
		tx = db
	}

	// 1. Audit & Normalize Invalid Statuses in user_lists
	fmt.Println("\n[1/6] Auditing user_lists watch statuses...")
	var invalidStatusEntries []models.UserMediaEntry
	tx.Where("status NOT IN (?) OR status IS NULL OR status = ''", []string{"watching", "completed", "plan_to_watch", "on_hold", "dropped"}).Find(&invalidStatusEntries)

	stats.InvalidStatusesFixed = int64(len(invalidStatusEntries))
	fmt.Printf("   Found %d entries with invalid status.\n", len(invalidStatusEntries))
	if !dryRun && len(invalidStatusEntries) > 0 {
		tx.Model(&models.UserMediaEntry{}).
			Where("status NOT IN (?) OR status IS NULL OR status = ''", []string{"watching", "completed", "plan_to_watch", "on_hold", "dropped"}).
			Update("status", "plan_to_watch")
		fmt.Println("   ✓ Normalized invalid statuses to 'plan_to_watch'.")
	}

	// 2. Audit & Clamp Ratings (Must be between 0.0 and 10.0)
	fmt.Println("\n[2/6] Auditing user ratings (< 0.0 or > 10.0)...")
	var invalidRatingEntries []models.UserMediaEntry
	tx.Where("rating < 0 OR rating > 10").Find(&invalidRatingEntries)

	stats.InvalidRatingsClamped = int64(len(invalidRatingEntries))
	fmt.Printf("   Found %d entries with out-of-bounds ratings.\n", len(invalidRatingEntries))
	if !dryRun && len(invalidRatingEntries) > 0 {
		tx.Model(&models.UserMediaEntry{}).Where("rating < 0").Update("rating", 0)
		tx.Model(&models.UserMediaEntry{}).Where("rating > 10").Update("rating", 10)
		fmt.Println("   ✓ Clamped ratings to valid 0.0 - 10.0 range.")
	}

	// 3. Normalize Visibility & Nullable metadata fields
	fmt.Println("\n[3/6] Normalizing privacy & visibility defaults...")
	var nullVisibilityCount int64
	tx.Model(&models.UserMediaEntry{}).Where("visibility_rating IS NULL OR visibility_rating = '' OR visibility_favorite IS NULL OR visibility_favorite = ''").Count(&nullVisibilityCount)

	stats.NullVisibilitiesFixed = nullVisibilityCount
	fmt.Printf("   Found %d entries with empty visibility settings.\n", nullVisibilityCount)
	if !dryRun && nullVisibilityCount > 0 {
		tx.Model(&models.UserMediaEntry{}).Where("visibility_rating IS NULL OR visibility_rating = ''").Update("visibility_rating", "public")
		tx.Model(&models.UserMediaEntry{}).Where("visibility_favorite IS NULL OR visibility_favorite = ''").Update("visibility_favorite", "public")
		fmt.Println("   ✓ Standardized visibility to 'public'.")
	}

	// 4. Audit & Prune Orphaned Records
	fmt.Println("\n[4/6] Auditing orphaned foreign-key relations...")

	// 4a. Orphan user_lists (User or Movie deleted)
	var orphanUserLists []models.UserMediaEntry
	tx.Raw(`
		SELECT ul.* FROM user_lists ul
		LEFT JOIN users u ON u.id = ul.user_id
		LEFT JOIN movies m ON m.id = ul.movie_id
		WHERE u.id IS NULL OR m.id IS NULL
	`).Scan(&orphanUserLists)
	stats.OrphanUserListsDeleted = int64(len(orphanUserLists))
	fmt.Printf("   Found %d orphaned user_lists records.\n", len(orphanUserLists))

	if !dryRun && len(orphanUserLists) > 0 {
		tx.Exec(`
			DELETE FROM user_lists WHERE id IN (
				SELECT ul.id FROM user_lists ul
				LEFT JOIN users u ON u.id = ul.user_id
				LEFT JOIN movies m ON m.id = ul.movie_id
				WHERE u.id IS NULL OR m.id IS NULL
			)
		`)
		fmt.Println("   ✓ Deleted orphaned user_lists records.")
	}

	// 4b. Orphan user_tv_progress (user_media_entry_id deleted)
	var orphanTVProgress []models.UserTVProgress
	tx.Raw(`
		SELECT utp.* FROM user_tv_progress utp
		LEFT JOIN user_lists ul ON ul.id = utp.user_media_entry_id
		WHERE ul.id IS NULL
	`).Scan(&orphanTVProgress)
	stats.OrphanTVProgressDeleted = int64(len(orphanTVProgress))
	fmt.Printf("   Found %d orphaned user_tv_progress records.\n", len(orphanTVProgress))

	if !dryRun && len(orphanTVProgress) > 0 {
		tx.Exec(`
			DELETE FROM user_tv_progress WHERE id IN (
				SELECT utp.id FROM user_tv_progress utp
				LEFT JOIN user_lists ul ON ul.id = utp.user_media_entry_id
				WHERE ul.id IS NULL
			)
		`)
		fmt.Println("   ✓ Deleted orphaned user_tv_progress records.")
	}

	// 4c. Orphan user_follows (follower or following deleted)
	var orphanFollows []models.UserFollow
	tx.Raw(`
		SELECT uf.* FROM user_follows uf
		LEFT JOIN users u1 ON u1.id = uf.follower_user_id
		LEFT JOIN users u2 ON u2.id = uf.following_user_id
		WHERE u1.id IS NULL OR u2.id IS NULL
	`).Scan(&orphanFollows)
	stats.OrphanFollowsDeleted = int64(len(orphanFollows))
	fmt.Printf("   Found %d orphaned user_follows records.\n", len(orphanFollows))

	if !dryRun && len(orphanFollows) > 0 {
		tx.Exec(`
			DELETE FROM user_follows WHERE id IN (
				SELECT uf.id FROM user_follows uf
				LEFT JOIN users u1 ON u1.id = uf.follower_user_id
				LEFT JOIN users u2 ON u2.id = uf.following_user_id
				WHERE u1.id IS NULL OR u2.id IS NULL
			)
		`)
		fmt.Println("   ✓ Deleted orphaned user_follows records.")
	}

	// 5. Sync TV progress & completed episode counters
	fmt.Println("\n[5/6] Auditing episode progress consistency...")
	var inconsistentProgress []models.UserMediaEntry
	tx.Where("status = 'completed' AND total_episodes > 0 AND episodes_watched < total_episodes").Find(&inconsistentProgress)
	stats.ProgressSynced = int64(len(inconsistentProgress))
	fmt.Printf("   Found %d completed entries with incomplete episode counts.\n", len(inconsistentProgress))

	if !dryRun && len(inconsistentProgress) > 0 {
		tx.Exec("UPDATE user_lists SET episodes_watched = total_episodes WHERE status = 'completed' AND total_episodes > 0 AND episodes_watched < total_episodes")
		fmt.Println("   ✓ Aligned watched episode counts for completed entries.")
	}

	// 6. Prune unused media cache (Optional)
	if pruneUnusedMedia {
		fmt.Println("\n[6/6] Pruning unused media cache...")
		var unusedMovies []models.Media
		tx.Raw(`
			SELECT m.* FROM movies m
			LEFT JOIN user_lists ul ON ul.movie_id = m.id
			WHERE ul.id IS NULL
		`).Scan(&unusedMovies)
		stats.UnusedMoviesPruned = int64(len(unusedMovies))
		fmt.Printf("   Found %d unreferenced cached movies.\n", len(unusedMovies))

		if !dryRun && len(unusedMovies) > 0 {
			tx.Exec(`
				DELETE FROM movies WHERE id IN (
					SELECT m.id FROM movies m
					LEFT JOIN user_lists ul ON ul.movie_id = m.id
					WHERE ul.id IS NULL
				)
			`)
			fmt.Println("   ✓ Pruned unreferenced media records.")
		}
	} else {
		fmt.Println("\n[6/6] Unused media prune skipped (use -prune-unused-media flag if desired).")
	}

	// Commit Transaction if not dry-run
	if !dryRun {
		if err := tx.Commit().Error; err != nil {
			log.Fatalf("Failed to commit database cleansing transaction: %v", err)
		}
		fmt.Println("\n✅ All database cleansing fixes committed successfully!")
	} else {
		fmt.Println("\nℹ️ Dry-run completed. No database records were modified.")
	}

	// Print Summary
	fmt.Println("\n================= CLEANSING SUMMARY =================")
	fmt.Printf(" Invalid Statuses Normalized : %d\n", stats.InvalidStatusesFixed)
	fmt.Printf(" Out-of-bounds Ratings Fixed : %d\n", stats.InvalidRatingsClamped)
	fmt.Printf(" Empty Visibilities Fixed    : %d\n", stats.NullVisibilitiesFixed)
	fmt.Printf(" Orphan User Lists Cleaned   : %d\n", stats.OrphanUserListsDeleted)
	fmt.Printf(" Orphan TV Progress Cleaned  : %d\n", stats.OrphanTVProgressDeleted)
	fmt.Printf(" Orphan Follows Cleaned      : %d\n", stats.OrphanFollowsDeleted)
	fmt.Printf(" Progress Counts Synced      : %d\n", stats.ProgressSynced)
	if pruneUnusedMedia {
		fmt.Printf(" Unused Media Pruned         : %d\n", stats.UnusedMoviesPruned)
	}
	fmt.Println("=====================================================")
}

func handleDatabaseReset(db *gorm.DB, confirmPhrase string) {
	if strings.TrimSpace(confirmPhrase) != "CONFIRM_RESET" {
		fmt.Println("\n❌ DATABASE RESET ABORTED!")
		fmt.Println("   To completely wipe and reset the database, you must pass:")
		fmt.Println("   go run cmd/cleanse/main.go -reset -confirm=CONFIRM_RESET")
		return
	}

	fmt.Println("\n⚠️  WARNING: RESETTING ALL DATABASE TABLES...")
	time.Sleep(1 * time.Second)

	err := db.Transaction(func(tx *gorm.DB) error {
		tx.Exec("TRUNCATE TABLE user_follows, user_tv_progress, user_lists, media_episodes, media_seasons, movies, users CASCADE;")
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to reset database: %v", err)
	}

	fmt.Println("✅ Database reset complete. All tables have been truncated and cleaned.")
}
