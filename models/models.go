package models

import "time"

type User struct {
    ID        uint       `gorm:"primaryKey"`
    Username  string     `gorm:"unique;not null"`
    Email     string     `gorm:"unique;not null"`
    Password  string     `gorm:"not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Movie struct {
    ID                uint   `gorm:"primaryKey"`
    TMDBID            int    `gorm:"uniqueIndex;not null"`
    Title             string `gorm:"not null"`
    Overview          string
    LocalPosterPath   string
    LocalBackdropPath string
    CreatedAt         time.Time
}

type UserList struct {
    ID        uint   `gorm:"primaryKey"`
    UserID    uint   `gorm:"not null;index"`
    MovieID   uint   `gorm:"not null;index"`
    Status    string `gorm:"type:varchar(20);default:'plan_to_watch'"`
    Rating    int    `gorm:"type:int;default:0"`
    CreatedAt time.Time
    UpdatedAt time.Time

    User  User  `gorm:"foreignKey:UserID"`
    Movie Movie `gorm:"foreignKey:MovieID"`
}