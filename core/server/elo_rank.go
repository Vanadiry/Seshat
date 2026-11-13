package server

import (
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/vanadiry/seshat/core/index"
	"github.com/vanadiry/seshat/core/store"
)

// updateELO 更新 ELO 评分并记录对战历史
func updateELO(dd string, winnerID, loserID int) {
	data := loadEloData()
	wb := data.Subjects[winnerID].Count
	lb := data.Subjects[loserID].Count
	k := (kFactor(wb) + kFactor(lb)) / 2.0

	wa := data.rating(winnerID)
	wbRating := data.rating(loserID)

	ea := 1.0 / (1.0 + math.Pow(10, (wbRating-wa)/400))
	eb := 1.0 / (1.0 + math.Pow(10, (wa-wbRating)/400))

	data.Subjects[winnerID] = eloSubjectData{
		Rating:        math.Round((wa+k*(1.0-ea))*10) / 10,
		Count:         wb + 1,
		LastCompareAt: data.GlobalCompareCount,
	}
	data.Subjects[loserID] = eloSubjectData{
		Rating:        math.Round((wbRating+k*(0.0-eb))*10) / 10,
		Count:         lb + 1,
		LastCompareAt: data.GlobalCompareCount,
	}
	data.GlobalCompareCount++

	saveEloData(data)

	history := loadELOHistory()
	history = append(history, eloHistory{Winner: winnerID, Loser: loserID, Time: time.Now().Format(time.RFC3339)})
	saveELOHistory(history)
}

// loadSubjectIndex 读取条目索引
func loadSubjectIndex(dd string) []eloEntry {
	list, _ := index.LoadCached[[]store.SubjectSummary](store.IndexFile(dd, "subjects.json"))
	var result []eloEntry
	for _, s := range list {
		result = append(result, eloEntry{ID: s.ID, Name: s.Name, NameCN: s.NameCN, Score: s.Score})
	}
	return result
}

// getELORanking 返回有 ELO、无 ELO 及孤立评分
func getELORanking(dd string) eloRankingResult {
	data := loadEloData()
	all := loadSubjectIndex(dd)
	seen := map[int]bool{}

	var entries []eloEntry
	var noRating []eloEntry
	for i := range all {
		if v, ok := data.Subjects[all[i].ID]; ok && v.Rating != 0 {
			r := v.Rating
			all[i].Rating = &r
			entries = append(entries, all[i])
		} else {
			noRating = append(noRating, all[i])
		}
		seen[all[i].ID] = true
	}
	sort.Slice(entries, func(i, j int) bool { return *entries[i].Rating > *entries[j].Rating })
	if n := len(entries); n > 1 {
		for i := range entries {
			pct := math.Round(float64(n-1-i)/float64(n-1)*100*10) / 10
			entries[i].RankPct = &pct
		}
	}
	sort.Slice(noRating, func(i, j int) bool { return noRating[i].Score > noRating[j].Score })

	var orphans []eloOrphan
	for id, v := range data.Subjects {
		if !seen[id] && v.Rating != 0 {
			orphans = append(orphans, eloOrphan{ID: id, Rating: v.Rating})
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Rating > orphans[j].Rating })

	if entries == nil {
		entries = []eloEntry{}
	}
	if noRating == nil {
		noRating = []eloEntry{}
	}
	if orphans == nil {
		orphans = []eloOrphan{}
	}
	return eloRankingResult{
		Total:         len(entries),
		Limit:         0,
		Offset:        0,
		Data:          entries,
		NoRating:      noRating,
		Orphans:       orphans,
		TotalCompares: data.GlobalCompareCount,
	}
}

// subjectSummary 返回轻量条目信息
type subjectInfo struct {
	Name   string
	NameCN string
	Score  float64
}

func subjectSummary(dd string, id int) subjectInfo {
	data, err := store.Get(dd, store.Key("subjects", id, "info.json"))
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
