package server

import (
	"math"
	"net/http"
)

func handleELOPair(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pair := getELOPair(dd)
		if pair == nil {
			writeError(w, r, http.StatusConflict, "至少需要 2 个本地条目")
			return
		}
		// pair 类型为 []eloPairEntry
		writeJSON(w, pair)
	}
}

func handleELOCompare(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Winner int `json:"winner"`
			Loser  int `json:"loser"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Winner == 0 || req.Loser == 0 {
			writeError(w, r, 400, "winner and loser required")
			return
		}
		updateELO(dd, req.Winner, req.Loser)
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleELORanking(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeJSON(w, getELORanking(dd)) }
}

func handleELOHistory(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeList(w, loadELOHistory()) }
}

func rebuildELO() {
	data := eloData{Subjects: map[int]eloSubjectData{}}
	history := loadELOHistory()
	for _, h := range history {
		wa := data.rating(h.Winner)
		wbRating := data.rating(h.Loser)
		wb := data.Subjects[h.Winner].Count
		lb := data.Subjects[h.Loser].Count
		k := (kFactor(wb) + kFactor(lb)) / 2.0

		ea := 1.0 / (1.0 + math.Pow(10, (wbRating-wa)/400))
		eb := 1.0 / (1.0 + math.Pow(10, (wa-wbRating)/400))

		data.Subjects[h.Winner] = eloSubjectData{
			Rating:        math.Round((wa+k*(1.0-ea))*10) / 10,
			Count:         wb + 1,
			LastCompareAt: data.GlobalCompareCount,
		}
		data.Subjects[h.Loser] = eloSubjectData{
			Rating:        math.Round((wbRating+k*(0.0-eb))*10) / 10,
			Count:         lb + 1,
			LastCompareAt: data.GlobalCompareCount,
		}
		data.GlobalCompareCount++
	}
	saveEloData(data)
}

func handleELORebuild(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rebuildELO()
		writeJSON(w, map[string]any{"status": "ok", "count": len(loadEloData().Subjects)})
	}
}
