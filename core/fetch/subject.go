package fetch

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

type subjectChar struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Relation string `json:"relation"`
	Actors   []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"actors"`
}

type subjectPerson struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Relation string `json:"relation"`
}

type subjectID struct {
	ID int `json:"id"`
}

// fetchAll 拉取单条目的信息、角色、人物、剧集、关联条目和图片
func fetchAll(sid int, bg *bangumi.Client, dd, imgDir string, p Progress, concurrency int, overwrite bool) bool {
	if taskCanceled(p) {
		return false
	}
	log.Info("fetching subject", "id", sid)
	var writeFailed atomic.Bool
	var requiredFailed atomic.Bool

	if !fetchSubjectInfo(dd, sid, bg, p, overwrite) {
		return false
	}

	chars, persons := fetchSubjectLists(dd, sid, bg, p, overwrite, &requiredFailed)

	charIDs := make([]subjectID, 0, len(chars))
	for _, c := range chars {
		charIDs = append(charIDs, subjectID{ID: c.ID})
	}
	personIDs := collectPersonIDs(chars, persons)
	log.Debug("subject details", "id", sid, "chars", len(charIDs), "persons_list", len(persons), "persons_total", len(personIDs))

	var wg sync.WaitGroup
	episodeCount := 0
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
				}
			}()
			defer wg.Done()
			fn()
		}()
	}
	run(func() { fetchCharacterInfos(charIDs, dd, bg, p, overwrite, concurrency, &writeFailed) })
	run(func() { fetchPersonInfos(personIDs, dd, bg, p, overwrite, concurrency, &writeFailed) })
	run(func() { fetchCharacterSubjects(charIDs, dd, bg, p, overwrite, concurrency, &writeFailed) })
	run(func() { fetchCharacterPersons(charIDs, dd, bg, p, overwrite, concurrency, &writeFailed) })
	run(func() { fetchPersonSubjects(personIDs, dd, bg, p, overwrite, concurrency, &writeFailed) })
	run(func() { fetchPersonCharacters(personIDs, dd, bg, p, overwrite, concurrency, &writeFailed) })
	run(func() {
		episodeCount = fetchSubjectEpisodes(sid, dd, bg, p, overwrite, &writeFailed, &requiredFailed)
	})
	run(func() { fetchSubjectRelations(sid, dd, bg, p, overwrite, &writeFailed, &requiredFailed) })
	wg.Wait()

	if writeFailed.Load() || requiredFailed.Load() {
		log.Warn("subject fetch incomplete", "id", sid, "write_failed", writeFailed.Load(), "required_failed", requiredFailed.Load())
		return false
	}
	log.Info("subject fetch done", "id", sid, "chars", len(charIDs), "persons", len(personIDs), "episodes", episodeCount)
	return true
}

func fetchSubjectInfo(dd string, sid int, bg *bangumi.Client, p Progress, overwrite bool) bool {
	if p != nil {
		p.Send("subject", 0, 1, "fetching")
	}
	_, err := loadOrFetchData(
		dd,
		store.Key("subjects", sid, "info.json"),
		fmt.Sprintf("v0/subjects/%d", sid),
		bg,
		overwrite,
	)
	if err != nil {
		if isCanceled(err) {
			return false
		}
		log.Warn("subject fetch failed", "id", sid, "err", err)
		if p != nil {
			p.SetError(err.Error())
			p.Send("subject", 1, 1, "error")
		}
		return false
	}
	if p != nil {
		p.Send("subject", 1, 1, "done")
	}
	return true
}

func fetchSubjectLists(dd string, sid int, bg *bangumi.Client, p Progress, overwrite bool, requiredFailed *atomic.Bool) ([]subjectChar, []subjectPerson) {
	var chars []subjectChar
	var persons []subjectPerson
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg.Done()
		data, err := loadOrFetchData(
			dd,
			store.Key("subjects", sid, "characters.json"),
			fmt.Sprintf("v0/subjects/%d/characters", sid),
			bg,
			overwrite,
		)
		if err != nil {
			if isCanceled(err) {
				return
			}
			log.Warn("subject characters fetch failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("角色列表拉取失败 #%d: %v", sid, err))
			}
			return
		}
		if err := json.Unmarshal(data, &chars); err != nil {
			log.Warn("unmarshal chars failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("角色列表解析失败 #%d: %v", sid, err))
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg.Done()
		data, err := loadOrFetchData(
			dd,
			store.Key("subjects", sid, "persons.json"),
			fmt.Sprintf("v0/subjects/%d/persons", sid),
			bg,
			overwrite,
		)
		if err != nil {
			if isCanceled(err) {
				return
			}
			log.Warn("subject persons fetch failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("人物列表拉取失败 #%d: %v", sid, err))
			}
			return
		}
		if err := json.Unmarshal(data, &persons); err != nil {
			log.Warn("unmarshal persons failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("人物列表解析失败 #%d: %v", sid, err))
			}
		}
	}()
	wg.Wait()
	return chars, persons
}

func collectPersonIDs(chars []subjectChar, persons []subjectPerson) []subjectID {
	personSet := map[int]bool{}
	for _, p := range persons {
		personSet[p.ID] = true
	}
	for _, c := range chars {
		for _, a := range c.Actors {
			if a.ID > 0 {
				personSet[a.ID] = true
			}
		}
	}
	ids := make([]subjectID, 0, len(personSet))
	for id := range personSet {
		ids = append(ids, subjectID{ID: id})
	}
	return ids
}

func fetchCharacterInfos(ids []subjectID, dd string, bg *bangumi.Client, p Progress, overwrite bool, concurrency int, writeFailed *atomic.Bool) {
	fetchConcurrent(ids, func(c subjectID) {
		key := store.Key("characters", c.ID, "info.json")
		if !overwrite && store.Has(dd, key) {
			return
		}
		data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID))
		if err != nil {
			log.Warn("character fetch failed", "id", c.ID, "err", err)
			p.SetError(err.Error())
			return
		}
		if !putData(dd, key, store.StripImages(data), p) {
			writeFailed.Store(true)
		}
	}, nil, concurrency)
}

func fetchPersonInfos(ids []subjectID, dd string, bg *bangumi.Client, p Progress, overwrite bool, concurrency int, writeFailed *atomic.Bool) {
	fetchConcurrent(ids, func(pp subjectID) {
		key := store.Key("persons", pp.ID, "info.json")
		if !overwrite && store.Has(dd, key) {
			return
		}
		data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID))
		if err != nil {
			log.Warn("person fetch failed", "id", pp.ID, "err", err)
			p.SetError(err.Error())
			return
		}
		if !putData(dd, key, store.StripImages(data), p) {
			writeFailed.Store(true)
		}
	}, nil, concurrency)
}

func fetchCharacterSubjects(ids []subjectID, dd string, bg *bangumi.Client, p Progress, overwrite bool, concurrency int, writeFailed *atomic.Bool) {
	fetchConcurrent(ids, func(c subjectID) {
		key := store.Key("characters", c.ID, "subjects.json")
		if !overwrite && store.Has(dd, key) {
			return
		}
		data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d/subjects", c.ID))
		if err != nil {
			return
		}
		if !putData(dd, key, store.StripImages(data), p) {
			writeFailed.Store(true)
		}
	}, nil, concurrency)
}

func fetchCharacterPersons(ids []subjectID, dd string, bg *bangumi.Client, p Progress, overwrite bool, concurrency int, writeFailed *atomic.Bool) {
	fetchConcurrent(ids, func(c subjectID) {
		key := store.Key("characters", c.ID, "persons.json")
		if !overwrite && store.Has(dd, key) {
			return
		}
		data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d/persons", c.ID))
		if err != nil {
			return
		}
		if !putData(dd, key, store.StripImages(data), p) {
			writeFailed.Store(true)
		}
	}, nil, concurrency)
}

func fetchPersonSubjects(ids []subjectID, dd string, bg *bangumi.Client, p Progress, overwrite bool, concurrency int, writeFailed *atomic.Bool) {
	fetchConcurrent(ids, func(pp subjectID) {
		key := store.Key("persons", pp.ID, "subjects.json")
		if !overwrite && store.Has(dd, key) {
			return
		}
		data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d/subjects", pp.ID))
		if err != nil {
			return
		}
		if !putData(dd, key, store.StripImages(data), p) {
			writeFailed.Store(true)
		}
	}, nil, concurrency)
}

func fetchPersonCharacters(ids []subjectID, dd string, bg *bangumi.Client, p Progress, overwrite bool, concurrency int, writeFailed *atomic.Bool) {
	fetchConcurrent(ids, func(pp subjectID) {
		key := store.Key("persons", pp.ID, "characters.json")
		if !overwrite && store.Has(dd, key) {
			return
		}
		data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d/characters", pp.ID))
		if err != nil {
			return
		}
		if !putData(dd, key, store.StripImages(data), p) {
			writeFailed.Store(true)
		}
	}, nil, concurrency)
}

func fetchSubjectEpisodes(sid int, dd string, bg *bangumi.Client, p Progress, overwrite bool, writeFailed, requiredFailed *atomic.Bool) int {
	epKey := store.Key("subjects", sid, "episodes.json")
	if !overwrite && store.Has(dd, epKey) {
		return 0
	}
	var allEps []json.RawMessage
	offset := 0
	complete := true
	for {
		data, err := bg.GetRaw(fmt.Sprintf("v0/episodes?subject_id=%d&limit=100&offset=%d", sid, offset))
		if err != nil {
			log.Warn("episodes fetch failed", "subject", sid, "offset", offset, "err", err)
			requiredFailed.Store(true)
			complete = false
			break
		}
		var page struct {
			Data  []json.RawMessage `json:"data"`
			Total int               `json:"total"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			log.Warn("episodes parse failed", "subject", sid, "offset", offset, "err", err)
			requiredFailed.Store(true)
			complete = false
			break
		}
		if len(page.Data) == 0 {
			if page.Total > 0 {
				log.Warn("episodes empty page", "subject", sid, "offset", offset, "total", page.Total)
				requiredFailed.Store(true)
				complete = false
			}
			break
		}
		allEps = append(allEps, page.Data...)
		if len(allEps) >= page.Total {
			break
		}
		offset += 100
	}
	if !complete {
		log.Warn("episodes incomplete, keep old file", "subject", sid, "fetched", len(allEps))
		return len(allEps)
	}
	if len(allEps) > 0 {
		result, err := json.Marshal(allEps)
		if err != nil {
			log.Error("marshal episodes failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			events.Bus.Error(fmt.Sprintf("剧集数据序列化失败 #%d", sid))
			return len(allEps)
		}
		if !putData(dd, store.Key("subjects", sid, "episodes.json"), store.StripImages(result), p) {
			writeFailed.Store(true)
		}
	}
	return len(allEps)
}

func fetchSubjectRelations(sid int, dd string, bg *bangumi.Client, p Progress, overwrite bool, writeFailed, requiredFailed *atomic.Bool) {
	relKey := store.Key("subjects", sid, "subjects.json")
	if !overwrite && store.Has(dd, relKey) {
		return
	}
	if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/subjects", sid)); err == nil {
		if !putData(dd, relKey, store.StripImages(data), p) {
			writeFailed.Store(true)
		}
	} else {
		log.Warn("subject relations fetch failed", "subject", sid, "err", err)
		requiredFailed.Store(true)
	}
}
