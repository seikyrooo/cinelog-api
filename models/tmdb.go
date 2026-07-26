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

type TMDBCast struct {
	Name      string `json:"name"`
	Character string `json:"character"`
}

type TMDBCrew struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

type TMDBCredits struct {
	Cast []TMDBCast `json:"cast"`
	Crew []TMDBCrew `json:"crew"`
}

type TMDBNextEpisode struct {
	AirDate       string `json:"air_date"`
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	SeasonNumber  int    `json:"season_number"`
}

type TMDBDetail struct {
	ID                 int              `json:"id"`
	Title              string           `json:"title,omitempty"`
	Name               string           `json:"name,omitempty"`
	Overview           string           `json:"overview"`
	PosterPath         string           `json:"poster_path"`
	BackdropPath       string           `json:"backdrop_path"`
	ReleaseDate        string           `json:"release_date,omitempty"`
	FirstAirDate       string           `json:"first_air_date,omitempty"`
	VoteAverage        float64          `json:"vote_average"`
	Status             string           `json:"status"`
	NumberOfSeasons    int              `json:"number_of_seasons"`
	NumberOfEpisodes   int              `json:"number_of_episodes"`
	NextEpisodeToAir   *TMDBNextEpisode `json:"next_episode_to_air"`
	Credits            *TMDBCredits     `json:"credits"`
	CreatedBy          []struct {
		Name string `json:"name"`
	} `json:"created_by"`
}