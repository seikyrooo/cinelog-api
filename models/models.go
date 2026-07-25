package models

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"unique;not null" json:"username"`
	Email     string    `gorm:"unique;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Movie struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	TMDBID            int       `gorm:"uniqueIndex:idx_tmdb_type;not null" json:"tmdb_id"`
	MediaType         string    `gorm:"uniqueIndex:idx_tmdb_type;type:varchar(10);default:'movie'" json:"media_type"` // "movie" or "tv"
	Title             string    `gorm:"not null" json:"title"`
	Overview          string    `gorm:"type:text" json:"overview"`
	ReleaseDate       string    `json:"release_date"`
	PosterPath        string    `json:"poster_path"`
	BackdropPath      string    `json:"backdrop_path"`
	LocalPosterPath   string    `json:"local_poster_path"`
	LocalBackdropPath string    `json:"local_backdrop_path"`
	VoteAverage       float64   `json:"vote_average"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type UserList struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"not null;index:idx_user_movie,unique" json:"user_id"`
	MovieID         uint      `gorm:"not null;index:idx_user_movie,unique" json:"movie_id"`
	Status          string    `gorm:"type:varchar(20);default:'plan_to_watch'" json:"status"` // plan_to_watch, watching, completed, on_hold, dropped
	Rating          float64   `gorm:"type:decimal(3,1);default:0.0" json:"rating"`
	Favorite        bool      `gorm:"default:false" json:"favorite"`
	Notes           string    `gorm:"type:text" json:"notes"`
	EpisodesWatched int       `gorm:"default:0" json:"episodes_watched"`
	TotalEpisodes   int       `gorm:"default:0" json:"total_episodes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	User  User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Movie Movie `gorm:"foreignKey:MovieID" json:"movie"`
}