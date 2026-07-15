package server

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/events"
	"github.com/vanadiry/seshat/Core/log"
)

const eloK = 32
const eloDefault = 1500

type eloEntry struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	NameCN  string   `json:"name_cn"`
	Score   float64  `json:"score"`
	Rating  *float64 `json:"rating,omitempty"`
	RankPct float64  `json:"rank_pct"`
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
	Winner int    `json:"winner"`
	Loser  int    `json:"loser"`
	Time   string `json:"time"`
}

func eloRatingPath() string {
	return filepath.Join(config.Dir(), "user", "elo", "rating.json")
}

func eloHistoryPath() string {
	return filepath.Join(config.Dir(), "user", "elo", "history.json")
}

func eloExcludePath() string {
	return filepath.Join(config.Dir(), "user", "elo", "exclude.toml")
}

func eloBattleCountsPath() string {
	return filepath.Join(config.Dir(), "user", "elo", "count.json")
}

const excludeTemplate = `# ELO 排除名单，此文件中的条目不会被列入评分配对
ids = []
`

// EnsureExcludeFile 确保 ELO 排除文件存在
func EnsureExcludeFile() {
	path := eloExcludePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(excludeTemplate), 0o644)
	}
}

func loadExcludeIDs() map[int]bool {
	data, err := os.ReadFile(eloExcludePath())
	if err != nil {
		return nil
	}
	ids := parseTOMLIDArray(data)
	if len(ids) == 0 {
		return nil
	}
	m := make(map[int]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
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
	data, err := json.Marshal(m)
	if err != nil {
		log.Error("saveELO marshal", "err", err)
		events.Bus.Error("ELO 评分保存失败")
		return
	}
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
	data, err := json.Marshal(h)
	if err != nil {
		log.Error("saveELOHistory marshal", "err", err)
		events.Bus.Error("ELO 历史保存失败")
		return
	}
	os.WriteFile(eloHistoryPath(), data, 0o644)
}

func loadBattleCounts() map[int]int {
	data, err := os.ReadFile(eloBattleCountsPath())
	if err != nil {
		return map[int]int{}
	}
	var m map[int]int
	json.Unmarshal(data, &m)
	if m == nil {
		m = map[int]int{}
	}
	return m
}

func saveBattleCounts(m map[int]int) {
	os.MkdirAll(filepath.Join(config.Dir(), "user", "elo"), 0o755)
	data, err := json.Marshal(m)
	if err != nil {
		log.Error("saveBattleCounts marshal", "err", err)
		events.Bus.Error("ELO 对战计数保存失败")
		return
	}
	os.WriteFile(eloBattleCountsPath(), data, 0o644)
}

// getELOPair 混合策略返回两个评比条目：
// 40% 低频优先，30% 相近分数，30% 随机
// 全部条目 ≥3 场后：20% 低频优先，50% 相近分数，30% 随机
func getELOPair(dd string) []eloPairEntry {
	keys, err := cache.List(dd, "subjects")
	if err != nil || len(keys) < 2 {
		return nil
	}
	excluded := loadExcludeIDs()
	var eligible []int
	for _, k := range keys {
		id, _ := strconv.Atoi(k)
		if !excluded[id] {
			eligible = append(eligible, id)
		}
	}
	if len(eligible) < 2 {
		return nil
	}

	counts := loadBattleCounts()

	// 检查是否全部条目 ≥3 场
	allSeen := true
	for _, id := range eligible {
		if counts[id] < 3 {
			allSeen = false
			break
		}
	}

	var lowPct, simPct float64
	if allSeen {
		lowPct, simPct = 0.2, 0.5
	} else {
		lowPct, simPct = 0.4, 0.3
	}

	r := rand.Float64()
	var id1, id2 int
	if r < lowPct {
		id1, id2 = pickLowFreq(eligible, counts)
	} else if r < lowPct+simPct {
		id1, id2 = pickSimilarScore(eligible)
	} else {
		id1, id2 = pickRandomPair(eligible)
	}

	info1 := subjectSummary(dd, id1)
	info2 := subjectSummary(dd, id2)
	return []eloPairEntry{
		{ID: id1, Name: info1.Name, NameCN: info1.NameCN},
		{ID: id2, Name: info2.Name, NameCN: info2.NameCN},
	}
}

// pickLowFreq 从最低对战次数的条目中选两个
func pickLowFreq(eligible []int, counts map[int]int) (int, int) {
	minCount := -1
	var pool []int
	for _, id := range eligible {
		c := counts[id]
		if minCount < 0 || c < minCount {
			minCount = c
			pool = []int{id}
		} else if c <= minCount+1 {
			pool = append(pool, id)
		}
	}
	i1 := rand.Intn(len(pool))
	i2 := rand.Intn(len(pool))
	for i2 == i1 {
		i2 = rand.Intn(len(pool))
	}
	return pool[i1], pool[i2]
}

// pickSimilarScore 随机选一个，再在 ±200 ELO 内选另一个
func pickSimilarScore(eligible []int) (int, int) {
	id1 := eligible[rand.Intn(len(eligible))]
	scores := loadELO()
	s1 := scores[id1]
	if s1 == 0 {
		s1 = eloDefault
	}
	var close []int
	for _, id := range eligible {
		if id == id1 {
			continue
		}
		s := scores[id]
		if s == 0 {
			s = eloDefault
		}
		if math.Abs(s-s1) <= 200 {
			close = append(close, id)
		}
	}
	if len(close) == 0 {
		return pickRandomPair(eligible)
	}
	return id1, close[rand.Intn(len(close))]
}

// pickRandomPair 随机选两个不同条目
func pickRandomPair(eligible []int) (int, int) {
	i1 := rand.Intn(len(eligible))
	i2 := rand.Intn(len(eligible))
	for i2 == i1 {
		i2 = rand.Intn(len(eligible))
	}
	return eligible[i1], eligible[i2]
}

// updateELO 更新 ELO 评分并记录对战历史
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

	// 累加对战计数
	counts := loadBattleCounts()
	counts[winnerID]++
	counts[loserID]++
	saveBattleCounts(counts)
}

// loadSubjectIndex 读取条目索引
func loadSubjectIndex(dd string) []eloEntry {
	list, _ := loadCachedIndex[[]cache.SubjectSummary](cache.IndexFile(dd, "subjects.json"))
	var result []eloEntry
	for _, s := range list {
		result = append(result, eloEntry{ID: s.ID, Name: s.Name, NameCN: s.NameCN, Score: s.Score})
	}
	return result
}

// getELORanking 返回有 ELO、无 ELO 及孤立评分
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
	if n := len(entries); n > 1 {
		for i := range entries {
			entries[i].RankPct = math.Round(float64(n-1-i)/float64(n-1)*100*10) / 10
		}
	}
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

// subjectSummary 返回轻量条目信息
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
		if pair == nil {
			writeError(w, http.StatusInternalServerError, "need at least 2 cached subjects")
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
		if limitBody(w, r); json.NewDecoder(r.Body).Decode(&req) != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Winner == 0 || req.Loser == 0 {
			writeError(w, 400, "winner and loser required")
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
	return func(w http.ResponseWriter, r *http.Request) { writeJSON(w, loadELOHistory()) }
}

func rebuildELO() {
	scores := map[int]float64{}
	counts := map[int]int{}
	history := loadELOHistory()
	for _, h := range history {
		wa := scores[h.Winner]
		if wa == 0 {
			wa = eloDefault
		}
		wb := scores[h.Loser]
		if wb == 0 {
			wb = eloDefault
		}
		ea := 1.0 / (1.0 + math.Pow(10, (wb-wa)/400))
		eb := 1.0 / (1.0 + math.Pow(10, (wa-wb)/400))
		scores[h.Winner] = wa + eloK*(1.0-ea)
		scores[h.Loser] = wb + eloK*(0.0-eb)
		counts[h.Winner]++
		counts[h.Loser]++
	}
	saveELO(scores)
	saveBattleCounts(counts)
}

func handleELORebuild(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rebuildELO()
		writeJSON(w, map[string]any{"status": "ok", "count": len(loadELO())})
	}
}
