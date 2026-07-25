package models

// Struktur data spesifik untuk hasil dari TMDB
type TMDBMovie struct {
    ID           int    `json:"id"`
    Title        string `json:"title"`
    Overview     string `json:"overview"`
    PosterPath   string `json:"poster_path"`
    BackdropPath string `json:"backdrop_path"`
    ReleaseDate  string `json:"release_date"`
}

type TMDBResponse struct {
    Results []TMDBMovie `json:"results"`
}