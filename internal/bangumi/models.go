package bangumi

type SubjectResponse struct {
	ID            int               `json:"id"`
	Type          int               `json:"type"`
	Name          string            `json:"name"`
	NameCN        string            `json:"name_cn"`
	Summary       string            `json:"summary"`
	Date          string            `json:"date"`
	Platform      string            `json:"platform"`
	Eps           int               `json:"eps"`
	TotalEpisodes int               `json:"total_episodes"`
	Volumes       int               `json:"volumes"`
	Series        bool              `json:"series"`
	Locked        bool              `json:"locked"`
	NSFW          bool              `json:"nsfw"`
	Images        SubjectImages     `json:"images"`
	Rating        SubjectRating     `json:"rating"`
	Collection    SubjectCollection `json:"collection"`
	Tags          []SubjectTag      `json:"tags"`
	Infobox       any               `json:"infobox"`
}

type SubjectImages struct {
	Large  string `json:"large"`
	Common string `json:"common"`
	Medium string `json:"medium"`
	Small  string `json:"small"`
	Grid   string `json:"grid"`
}

type SubjectRating struct {
	Rank  int            `json:"rank"`
	Total int            `json:"total"`
	Count map[string]int `json:"count"`
	Score float64        `json:"score"`
}

type SubjectCollection struct{ Wish, Collect, Doing, OnHold, Dropped int }
type SubjectTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type PersonResponse struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	Type         int          `json:"type"`
	Career       []string     `json:"career"`
	Summary      string       `json:"summary"`
	Locked       bool         `json:"locked"`
	LastModified string       `json:"last_modified"`
	Images       PersonImages `json:"images"`
	Infobox      any          `json:"infobox"`
	Gender       string       `json:"gender"`
	BloodType    int          `json:"blood_type"`
	BirthYear    int          `json:"birth_year"`
	BirthMon     int          `json:"birth_mon"`
	BirthDay     int          `json:"birth_day"`
	Stat         Stat         `json:"stat"`
}

type PersonImages struct {
	Large  string `json:"large"`
	Medium string `json:"medium"`
	Small  string `json:"small"`
	Grid   string `json:"grid"`
}

type Stat struct{ Comments, Collects int }

type CharacterResponse struct {
	ID        int          `json:"id"`
	Name      string       `json:"name"`
	Type      int          `json:"type"`
	Summary   string       `json:"summary"`
	Locked    bool         `json:"locked"`
	NSFW      bool         `json:"nsfw"`
	Images    PersonImages `json:"images"`
	Infobox   any          `json:"infobox"`
	Gender    string       `json:"gender"`
	BloodType int          `json:"blood_type"`
	BirthYear int          `json:"birth_year"`
	BirthMon  int          `json:"birth_mon"`
	BirthDay  int          `json:"birth_day"`
	Stat      Stat         `json:"stat"`
}

type RelatedCharacter struct {
	ID       int          `json:"id"`
	Name     string       `json:"name"`
	Type     int          `json:"type"`
	Relation string       `json:"relation"`
	Images   PersonImages `json:"images"`
	Actors   []Actor      `json:"actors"`
}

type Actor struct {
	ID      int          `json:"id"`
	Name    string       `json:"name"`
	Type    int          `json:"type"`
	Career  []string     `json:"career"`
	Images  PersonImages `json:"images"`
	Summary string       `json:"short_summary"`
	Locked  bool         `json:"locked"`
}

type RelatedPerson struct {
	ID       int          `json:"id"`
	Name     string       `json:"name"`
	Type     int          `json:"type"`
	Career   []string     `json:"career"`
	Relation string       `json:"relation"`
	Eps      string       `json:"eps"`
	Images   PersonImages `json:"images"`
}

type EpisodeResponse struct {
	ID       int     `json:"id"`
	Type     int     `json:"type"`
	Sort     float64 `json:"sort"`
	Ep       float64 `json:"ep,omitempty"`
	Name     string  `json:"name"`
	NameCN   string  `json:"name_cn"`
	Duration string  `json:"duration"`
	Airdate  string  `json:"airdate"`
	Desc     string  `json:"desc"`
	Disc     int     `json:"disc"`
}

type PagedResponse struct{ Total, Limit, Offset int }

type PagedEpisodes struct {
	PagedResponse
	Data []EpisodeResponse `json:"data"`
}

type SubjectRelationResponse struct {
	ID       int           `json:"id"`
	Type     int           `json:"type"`
	Name     string        `json:"name"`
	NameCN   string        `json:"name_cn"`
	Images   SubjectImages `json:"images"`
	Relation string        `json:"relation"`
}
