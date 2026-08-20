package models

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"unique;not null" json:"username"`
	Email     string    `gorm:"unique;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	Bio       string    `gorm:"type:text" json:"bio"`
	AvatarURL string    `json:"avatar_url"`
	IsPublic  bool      `gorm:"default:true" json:"is_public"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Media adalah nama domain baru untuk cache metadata film/TV.
// Untuk kompatibilitas transisi, model ini tetap dipetakan ke tabel legacy `movies`.
type Media struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	TMDBID            int       `gorm:"uniqueIndex:idx_tmdb_type;not null" json:"tmdb_id"`
	MediaType         string    `gorm:"uniqueIndex:idx_tmdb_type;type:varchar(10);default:'movie'" json:"media_type"`
	Title             string    `gorm:"not null" json:"title"`
	OriginalTitle     string    `json:"original_title"`
	Overview          string    `gorm:"type:text" json:"overview"`
	ReleaseDate       string    `json:"release_date"`
	FirstAirDate      string    `json:"first_air_date"`
	PosterPath        string    `json:"poster_path"`
	BackdropPath      string    `json:"backdrop_path"`
	LocalPosterPath   string    `json:"local_poster_path"`
	LocalBackdropPath string    `json:"local_backdrop_path"`
	Genres            string    `gorm:"type:text" json:"genres"`
	VoteAverage       float64   `json:"vote_average"`
	Popularity        float64   `json:"popularity"`
	Director          string    `json:"director"`
	Cast              string    `json:"cast"`
	TotalSeasons      int       `gorm:"default:0" json:"total_seasons"`
	TotalEpisodes     int       `gorm:"default:0" json:"total_episodes"`
	NextAirDate       string    `json:"next_air_date"`
	NextEpisodeName   string    `json:"next_episode_name"`
	MediaStatus       string    `json:"media_status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (Media) TableName() string {
	return "movies"
}

type MediaSeason struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	MediaID      uint      `gorm:"not null;index" json:"media_id"`
	SeasonNumber int       `gorm:"not null" json:"season_number"`
	Name         string    `json:"name"`
	EpisodeCount int       `gorm:"default:0" json:"episode_count"`
	AirDate      string    `json:"air_date"`
	Overview     string    `gorm:"type:text" json:"overview"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (MediaSeason) TableName() string {
	return "media_seasons"
}

type MediaEpisode struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	MediaID       uint      `gorm:"not null;index" json:"media_id"`
	SeasonNumber  int       `gorm:"not null" json:"season_number"`
	EpisodeNumber int       `gorm:"not null" json:"episode_number"`
	Title         string    `json:"title"`
	Overview      string    `gorm:"type:text" json:"overview"`
	AirDate       string    `json:"air_date"`
	Runtime       int       `gorm:"default:0" json:"runtime"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (MediaEpisode) TableName() string {
	return "media_episodes"
}

// UserMediaEntry adalah nama domain baru untuk relasi personal user terhadap satu media.
// Untuk menjaga flow lama tetap jalan, model ini masih memakai tabel legacy `user_lists`.
type UserMediaEntry struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	UserID             uint       `gorm:"not null;index:idx_user_movie,unique" json:"user_id"`
	MovieID            uint       `gorm:"not null;index:idx_user_movie,unique" json:"movie_id"`
	Status             string     `gorm:"type:varchar(20);default:'plan_to_watch'" json:"status"`
	Rating             float64    `gorm:"type:decimal(3,1);default:0.0" json:"rating"`
	Favorite           bool       `gorm:"default:false" json:"favorite"`
	Notes              string     `gorm:"type:text" json:"notes"`
	Review             string     `gorm:"type:text" json:"review"`
	VisibilityRating   string     `gorm:"type:varchar(20);default:'public'" json:"visibility_rating"`
	VisibilityFavorite string     `gorm:"type:varchar(20);default:'public'" json:"visibility_favorite"`
	SeasonWatched      int        `gorm:"default:1" json:"season_watched"`
	EpisodesWatched    int        `gorm:"default:0" json:"episodes_watched"`
	TotalEpisodes      int        `gorm:"default:0" json:"total_episodes"`
	LastWatchedAt      *time.Time `json:"last_watched_at"`
	StartedAt          *time.Time `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	User  User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Movie Movie `gorm:"foreignKey:MovieID" json:"movie"`
}

func (UserMediaEntry) TableName() string {
	return "user_lists"
}

type UserTVProgress struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	UserMediaEntryID uint       `gorm:"not null;uniqueIndex" json:"user_media_entry_id"`
	CurrentSeason    int        `gorm:"default:1" json:"current_season"`
	CurrentEpisode   int        `gorm:"default:0" json:"current_episode"`
	EpisodesWatched  int        `gorm:"default:0" json:"episodes_watched_count"`
	LastWatchedAt    *time.Time `json:"last_watched_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (UserTVProgress) TableName() string {
	return "user_tv_progress"
}

type UserFollow struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	FollowerUserID  uint      `gorm:"not null;uniqueIndex:idx_user_follows" json:"follower_user_id"`
	FollowingUserID uint      `gorm:"not null;uniqueIndex:idx_user_follows" json:"following_user_id"`
	CreatedAt       time.Time `json:"created_at"`

	Follower  User `gorm:"foreignKey:FollowerUserID" json:"follower,omitempty"`
	Following User `gorm:"foreignKey:FollowingUserID" json:"following,omitempty"`
}

func (UserFollow) TableName() string {
	return "user_follows"
}

type ActivityLike struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `gorm:"not null;uniqueIndex:idx_user_entry_like" json:"user_id"`
	UserMediaEntryID uint      `gorm:"not null;uniqueIndex:idx_user_entry_like" json:"user_media_entry_id"`
	CreatedAt        time.Time `json:"created_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (ActivityLike) TableName() string {
	return "activity_likes"
}

type ActivityComment struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `gorm:"not null;index" json:"user_id"`
	UserMediaEntryID uint      `gorm:"not null;index" json:"user_media_entry_id"`
	Content          string    `gorm:"type:text;not null" json:"content"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (ActivityComment) TableName() string {
	return "activity_comments"
}

type Notification struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	RecipientUserID  uint      `gorm:"not null;index" json:"recipient_user_id"`
	ActorUserID      uint      `gorm:"not null" json:"actor_user_id"`
	UserMediaEntryID uint      `json:"user_media_entry_id,omitempty"`
	Type             string    `gorm:"type:varchar(30);not null" json:"type"` // "like", "comment", "follow"
	Message          string    `gorm:"type:text" json:"message"`
	IsRead           bool      `gorm:"default:false;index" json:"is_read"`
	CreatedAt        time.Time `json:"created_at"`

	Actor User `gorm:"foreignKey:ActorUserID" json:"actor,omitempty"`
}

func (Notification) TableName() string {
	return "notifications"
}

// Legacy aliases agar controller lama tetap bisa memakai naming saat ini.
type Movie = Media
type UserList = UserMediaEntry
