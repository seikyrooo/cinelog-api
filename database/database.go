package database

import (
    "log"
    "os"

    "cinelog-api/models"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
    dsn := os.Getenv("DB_DSN")

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Info),
    })

    if err != nil {
        log.Fatal("Gagal terhubung ke database! \n", err)
    }

    log.Println("Berhasil terhubung ke database.")

    // Auto Migrate skema tabel
    err = db.AutoMigrate(&models.User{}, &models.Movie{}, &models.UserList{})
    if err != nil {
        log.Fatal("Gagal migrasi database! \n", err)
    }

    log.Println("Database Migration selesai.")
    DB = db
}