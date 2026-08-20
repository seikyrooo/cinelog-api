package database

import (
	"log"
	"os"
	"strings"
	"time"

	"cinelog-api/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	dsn := strings.TrimSpace(os.Getenv("DB_DSN"))

	var dialector gorm.Dialector
	if dsn == "" || strings.HasSuffix(dsn, ".db") || strings.HasPrefix(dsn, "sqlite:") {
		dbFile := dsn
		if dbFile == "" || strings.HasPrefix(dbFile, "sqlite:") {
			dbFile = "cinelog.db"
		}
		dialector = sqlite.Open(dbFile)
		log.Printf("Connecting to SQLite database: %s", dbFile)
	} else {
		dialector = postgres.Open(dsn)
		log.Println("Connecting to PostgreSQL database...")
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Gagal terhubung ke database! \n", err)
	}

	log.Println("Berhasil terhubung ke database.")

	// Configure connection pooling for high throughput and connection reuse
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxIdleConns(20)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(1 * time.Hour)
		sqlDB.SetConnMaxIdleTime(15 * time.Minute)
	}

	// Auto Migrate skema tabel.
	// Domain model baru (Media, UserMediaEntry) tetap diarahkan ke tabel legacy
	// agar data existing dan controller lama tetap kompatibel selama fase transisi.
	err = db.AutoMigrate(
		&models.User{},
		&models.Media{},
		&models.MediaSeason{},
		&models.MediaEpisode{},
		&models.UserMediaEntry{},
		&models.UserTVProgress{},
		&models.UserFollow{},
		&models.ActivityLike{},
		&models.ActivityComment{},
		&models.Notification{},
	)
	if err != nil {
		log.Fatal("Gagal migrasi database! \n", err)
	}

	// Backfill any NULL or unset is_public in users table to true
	_ = db.Exec("UPDATE users SET is_public = true WHERE is_public IS NULL").Error

	// Clean up existing usernames with spaces by replacing spaces with underscores
	_ = db.Exec("UPDATE users SET username = REPLACE(TRIM(username), ' ', '_') WHERE username LIKE '% %'").Error

	log.Println("Database Migration selesai.")
	DB = db
}
