package models

type TMDBMedia struct {
	ID           int     `json:"id"`
	Title        string  `json:"title,omitempty"`
	Name         string  `json:"name,omitempty"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date,omitempty"`
	FirstAirDate string  `json:"first_air_date,omitempty"`
	MediaType    string  `json:"media_type,omitempty"`
	VoteAverage  float64 `json:"vote_average"`
}

type TMDBResponse struct {
	Page         int         `json:"page"`
	Results      []TMDBMedia `json:"results"`
	TotalPages   int         `json:"total_pages"`
	TotalResults int         `json:"total_results"`
}