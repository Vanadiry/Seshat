package model

// Subject 表示一部 Bangumi 动画条目。
type Subject struct {
	ID            int     `json:"id"`
	Type          int     `json:"type"`
	Name          string  `json:"name"`
	NameCN        string  `json:"name_cn"`
	Summary       string  `json:"summary"`
	Date          string  `json:"date,omitempty"`
	Platform      string  `json:"platform"`
	Eps           int     `json:"eps"`
	TotalEpisodes int     `json:"total_episodes"`
	Volumes       int     `json:"volumes"`
	Series        bool    `json:"series"`
	Locked        bool    `json:"locked"`
	NSFW          bool    `json:"nsfw"`
	Score         float64 `json:"score,omitempty"`
	Rank          int     `json:"rank,omitempty"`
	RatingTotal   int     `json:"rating_total,omitempty"`
	WishCount     int     `json:"wish_count"`
	CollectCount  int     `json:"collect_count"`
	DoingCount    int     `json:"doing_count"`
	OnHoldCount   int     `json:"on_hold_count"`
	DroppedCount  int     `json:"dropped_count"`
	ImagePath     string  `json:"image_path"`
	Infobox       string  `json:"infobox"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type Character struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Type          int    `json:"type"`
	Summary       string `json:"summary"`
	Gender        string `json:"gender,omitempty"`
	BloodType     int    `json:"blood_type,omitempty"`
	BirthYear     int    `json:"birth_year,omitempty"`
	BirthMon      int    `json:"birth_mon,omitempty"`
	BirthDay      int    `json:"birth_day,omitempty"`
	Locked        bool   `json:"locked"`
	NSFW          bool   `json:"nsfw"`
	ImagePath     string `json:"image_path"`
	ImageGridPath string `json:"image_grid_path"`
	Infobox       string `json:"infobox"`
	CommentCount  int    `json:"comment_count"`
	CollectCount  int    `json:"collect_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type Person struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Type          int    `json:"type"`
	Summary       string `json:"summary"`
	Gender        string `json:"gender,omitempty"`
	BloodType     int    `json:"blood_type,omitempty"`
	BirthYear     int    `json:"birth_year,omitempty"`
	BirthMon      int    `json:"birth_mon,omitempty"`
	BirthDay      int    `json:"birth_day,omitempty"`
	Locked        bool   `json:"locked"`
	ImagePath     string `json:"image_path"`
	ImageGridPath string `json:"image_grid_path"`
	Infobox       string `json:"infobox"`
	Career        string `json:"career"`
	CommentCount  int    `json:"comment_count"`
	CollectCount  int    `json:"collect_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type Episode struct {
	ID        int     `json:"id"`
	SubjectID int     `json:"subject_id"`
	Type      int     `json:"type"`
	Sort      float64 `json:"sort"`
	Ep        float64 `json:"ep,omitempty"`
	Name      string  `json:"name"`
	NameCN    string  `json:"name_cn"`
	Duration  string  `json:"duration"`
	Airdate   string  `json:"airdate,omitempty"`
	Desc      string  `json:"desc"`
	Disc      int     `json:"disc"`
}

type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count,omitempty"`
}

type SubjectCharacter struct {
	SubjectID   int                    `json:"subject_id"`
	CharacterID int                    `json:"character_id"`
	Relation    string                 `json:"relation"`
	Character   *Character             `json:"character,omitempty"`
	Actors      []SubjectCharacterActor `json:"actors,omitempty"`
}

type SubjectCharacterActor struct {
	PersonID int    `json:"person_id"`
	Name     string `json:"name"`
}

type SubjectPerson struct {
	SubjectID int     `json:"subject_id"`
	PersonID  int     `json:"person_id"`
	Relation  string  `json:"relation"`
	Eps       string  `json:"eps"`
	Person    *Person `json:"person,omitempty"`
}

type SubjectRelation struct {
	SubjectID int    `json:"subject_id"`
	RelatedID int    `json:"related_id"`
	Relation  string `json:"relation"`
}

type CustomField struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type UserSubjectData struct {
	SubjectID int               `json:"subject_id"`
	Fields    map[string]string `json:"fields"`
}

type Collection struct {
	SubjectID int    `json:"subject_id"`
	Type      int    `json:"type"`
	Rate      int    `json:"rate"`
	Comment   string `json:"comment,omitempty"`
	Tags      string `json:"tags"`
	Private   bool   `json:"private"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type EloComparison struct {
	ID        int    `json:"id"`
	WinnerID  int    `json:"winner_id"`
	LoserID   int    `json:"loser_id"`
	CreatedAt string `json:"created_at"`
}

type EloRating struct {
	SubjectID int     `json:"subject_id"`
	Rating    float64 `json:"rating"`
	UpdatedAt string  `json:"updated_at"`
}

type APIResponse struct {
	Data  any       `json:"data,omitempty"`
	Total *int      `json:"total,omitempty"`
	Page  *int      `json:"page,omitempty"`
	Limit *int      `json:"limit,omitempty"`
	Error *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
