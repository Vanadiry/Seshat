package server

import (
	"net/http"
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
)

const eloK = 32
const eloDefault = 1500

type eloEntry struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	NameCN string   `json:"name_cn"`
	Score  float64  `json:"score"`
	Rating *float64 `json:"rating,omitempty"`
}

type eloOrphan struct {
	ID     int     `json:"id"`
	Rating float64 `json:"rating"`
}

type eloRankingResult struct {
	Entries  []eloEntry  `json:"entries"`
	NoRating []eloEntry  `json:"no_rating"`
	Orphans  []eloOrphan `json:"orphans"`
}

type eloPairEntry struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NameCN string `json:"name_cn"`
}

type eloHistory struct {
	Winner  int    `json:"winner"`
	Loser   int    `json:"loser"`
	Time    string `json:"time"`
}

func eloRatingPath() string {
	return filepath.Join(config.Dir(), "user", "elo", "rating.json")
}

func eloHistoryPath() string {
	return filepath.Join(config.Dir(), "user", "elo", "history.json")
}

func loadELO() map[int]float64 {
	data, err := os.ReadFile(eloRatingPath())
	if err != nil {
		return map[int]float64{}
	}
	var m map[int]float64
	json.Unmarshal(data, &m)
	if m == nil {
		m = map[int]float64{}
	}
	return m
}

func saveELO(m map[int]float64) {
	os.MkdirAll(filepath.Join(config.Dir(), "user", "elo"), 0o755)
	data, _ := json.Marshal(m)
	os.WriteFile(eloRatingPath(), data, 0o644)
}

func loadELOHistory() []eloHistory {
	data, err := os.ReadFile(eloHistoryPath())
	if err != nil {
		return []eloHistory{}
	}
	var h []eloHistory
	json.Unmarshal(data, &h)
	if h == nil {
		h = []eloHistory{}
	}
	return h
}

func saveELOHistory(h []eloHistory) {
	os.MkdirAll(filepath.Join(config.Dir(), "user", "elo"), 0o755)
	data, _ := json.Marshal(h)
	os.WriteFile(eloHistoryPath(), data, 0o644)
}

// getELOPair returns two random cached subjects for comparison.
func getELOPair(dd string) []eloPairEntry {
	keys, err := cache.List(dd, "subjects")
	if err != nil || len(keys) < 2 {
		return nil
	}
	i1 := rand.Intn(len(keys))
	i2 := rand.Intn(len(keys))
	for i2 == i1 {
		i2 = rand.Intn(len(keys))
	}
	id1, _ := strconv.Atoi(keys[i1])
	id2, _ := strconv.Atoi(keys[i2])
	info1 := subjectSummary(dd, id1)
	info2 := subjectSummary(dd, id2)
	return []eloPairEntry{
		{ID: id1, Name: info1.Name, NameCN: info1.NameCN},
		{ID: id2, Name: info2.Name, NameCN: info2.NameCN},
	}
}

// updateELO updates ratings after a comparison and records the battle in history.
func updateELO(dd string, winnerID, loserID int) {
	scores := loadELO()
	wa := scores[winnerID]
	if wa == 0 {
		wa = eloDefault
	}
	wb := scores[loserID]
	if wb == 0 {
		wb = eloDefault
	}

	ea := 1.0 / (1.0 + math.Pow(10, (wb-wa)/400))
	eb := 1.0 / (1.0 + math.Pow(10, (wa-wb)/400))

	scores[winnerID] = wa + eloK*(1.0-ea)
	scores[loserID] = wb + eloK*(0.0-eb)

	saveELO(scores)

	history := loadELOHistory()
	history = append(history, eloHistory{Winner: winnerID, Loser: loserID, Time: time.Now().Format(time.RFC3339)})
	saveELOHistory(history)
}

// loadSubjectIndex reads subjects.json index for fast lookup.
func loadSubjectIndex(dd string) []eloEntry {
	data, err := os.ReadFile(cache.IndexFile(dd, "subjects.json"))
	if err != nil {
		return nil
	}
	var list []struct {
		ID     int     `json:"id"`
		Name   string  `json:"name"`
		NameCN string  `json:"name_cn"`
		Score  float64 `json:"score"`
	}
	json.Unmarshal(data, &list)
	var result []eloEntry
	for _, s := range list {
		result = append(result, eloEntry{ID: s.ID, Name: s.Name, NameCN: s.NameCN, Score: s.Score})
	}
	return result
}

// getELORanking returns entries with ELO, entries without ELO, and orphan scores.
func getELORanking(dd string) eloRankingResult {
	scores := loadELO()
	all := loadSubjectIndex(dd)
	seen := map[int]bool{}

	var entries []eloEntry
	var noRating []eloEntry
	for i := range all {
		if v, ok := scores[all[i].ID]; ok {
			r := v
			all[i].Rating = &r
			entries = append(entries, all[i])
		} else {
			noRating = append(noRating, all[i])
		}
		seen[all[i].ID] = true
	}
	sort.Slice(entries, func(i, j int) bool { return *entries[i].Rating > *entries[j].Rating })
	sort.Slice(noRating, func(i, j int) bool { return noRating[i].Score > noRating[j].Score })

	var orphans []eloOrphan
	for id, rating := range scores {
		if !seen[id] && rating != 0 {
			orphans = append(orphans, eloOrphan{ID: id, Rating: rating})
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Rating > orphans[j].Rating })

	return eloRankingResult{Entries: entries, NoRating: noRating, Orphans: orphans}
}

// subjectSummary returns a lightweight subject entry (only Score is used externally).
type subjectInfo struct {
	Name   string
	NameCN string
	Score  float64
}

func subjectSummary(dd string, id int) subjectInfo {
	data, err := cache.Get(dd, cache.Key("subjects", id, "info.json"))
	if err != nil {
		return subjectInfo{}
	}
	var s struct {
		Name   string `json:"name"`
		NameCN string `json:"name_cn"`
		Rating struct {
			Score float64 `json:"score"`
		} `json:"rating"`
	}
	json.Unmarshal(data, &s)
	return subjectInfo{Name: s.Name, NameCN: s.NameCN, Score: s.Rating.Score}
}

func handleELOPair(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pair := getELOPair(dd)
		if pair == nil { writeJSON(w, map[string]string{"error": "need at least 2 cached subjects"}); return }
		// pair is []eloPairEntry — ensure we handle it correctly
		writeJSON(w, pair)
	}
}

func handleELOCompare(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Winner int `json:"winner"`; Loser int `json:"loser"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Winner == 0 || req.Loser == 0 { http.Error(w, `{"error":"winner and loser required"}`, 400); return }
		updateELO(dd, req.Winner, req.Loser)
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleELORanking(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeJSON(w, getELORanking(dd)) }
}

func handleELOHistory(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeJSON(w, loadELOHistory()) }
}

func rebuildELO() {
	scores := map[int]float64{}
	history := loadELOHistory()
	for _, h := range history {
		wa := scores[h.Winner]
		if wa == 0 { wa = eloDefault }
		wb := scores[h.Loser]
		if wb == 0 { wb = eloDefault }
		ea := 1.0 / (1.0 + math.Pow(10, (wb-wa)/400))
		eb := 1.0 / (1.0 + math.Pow(10, (wa-wb)/400))
		scores[h.Winner] = wa + eloK*(1.0-ea)
		scores[h.Loser] = wb + eloK*(0.0-eb)
	}
	saveELO(scores)
}

func handleELORebuild(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rebuildELO()
		writeJSON(w, map[string]any{"status": "ok", "count": len(loadELO())})
	}
}
