package fetch

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/vanadiry/seshat/internal/bangumi"
	"github.com/vanadiry/seshat/internal/db"
	"github.com/vanadiry/seshat/internal/log"
	"github.com/vanadiry/seshat/internal/task"
)

type Service struct {
	Client      *bangumi.Client
	DB          *sql.DB
	DataDir     string
	Concurrency int
	Tasks       *task.Manager
}

// FetchSubject 拉取一部动画的全部数据并通过 task 推送进度。
func (s *Service) FetchSubject(t *task.Task) {
	concurrency := s.Concurrency
	if concurrency < 1 {
		concurrency = 32
	}

	log.Info("Fetching subject #%d", t.SubjectID)
	t.Send(`{"step":"subject","status":"fetching"}`)
	subj, err := s.Client.GetSubject(t.SubjectID)
	if err != nil {
		t.Status = task.StatusFailed
		t.Error = err.Error()
		t.Close()
		return
	}

	infoboxJSON, _ := json.Marshal(subj.Infobox)
	tx, _ := s.DB.Begin()
	db.DeleteSubjectCascade(tx, t.SubjectID)
	db.UpsertSubject(tx, subj.ID, subj.Name, subj.NameCN, subj.Summary, subj.Date, subj.Platform,
		subj.Eps, subj.TotalEpisodes, subj.Volumes, subj.Series, subj.Locked, subj.NSFW,
		subj.Rating.Score, subj.Rating.Rank, subj.Rating.Total,
		subj.Collection.Wish, subj.Collection.Collect, subj.Collection.Doing,
		subj.Collection.OnHold, subj.Collection.Dropped, "", string(infoboxJSON))
	for _, tg := range subj.Tags {
		db.UpsertTag(tx, tg.Name)
		db.UpsertSubjectTag(tx, subj.ID, tg.Name, tg.Count)
	}
	tx.Commit()

	chars, _ := s.Client.GetSubjectCharacters(t.SubjectID)
	log.Info("  characters: %d", len(chars))
	t.Send(fmt.Sprintf(`{"step":"characters","done":0,"total":%d}`, len(chars)))
	s.pipeChars(t.SubjectID, chars, concurrency, t)

	persons, _ := s.Client.GetSubjectPersons(t.SubjectID)
	log.Info("  persons: %d", len(persons))
	t.Send(fmt.Sprintf(`{"step":"persons","done":0,"total":%d}`, len(persons)))
	s.pipePersons(t.SubjectID, persons, concurrency, t)

	t.Send(`{"step":"episodes","status":"fetching"}`)
	s.fetchEpisodes(t.SubjectID)
	t.Send(`{"step":"episodes","status":"done"}`)

	t.Send(`{"step":"relations","status":"fetching"}`)
	s.fetchRelations(t.SubjectID)
	t.Send(`{"step":"relations","status":"done"}`)

	t.Send(`{"step":"image","status":"downloading"}`)
	if p, err := DownloadImage(subj.Images.Large, s.DataDir, t.SubjectID, "subject"); err == nil {
		s.DB.Exec(`UPDATE subject SET image_path = ? WHERE id = ?`, p, t.SubjectID)
	}
	if p, err := DownloadImage(subj.Images.Grid, s.DataDir, t.SubjectID, "subject_grid"); err == nil {
		s.DB.Exec(`UPDATE subject SET image_grid_path = ? WHERE id = ?`, p, t.SubjectID)
	}
	t.Send(`{"step":"image","status":"done"}`)

	log.Info("Subject #%d fetch complete", t.SubjectID)
	t.Status = task.StatusComplete
	t.Send(`{"step":"complete"}`)
	t.Close()
}

type charJob struct{ rc bangumi.RelatedCharacter; detail *bangumi.CharacterResponse }

func (s *Service) pipeChars(subjectID int, chars []bangumi.RelatedCharacter, concurrency int, t *task.Task) {
	results := make(chan charJob, len(chars))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, rc := range chars {
		wg.Add(1)
		go func(rc bangumi.RelatedCharacter) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detail, err := s.Client.GetCharacter(rc.ID)
			if err != nil {
				detail = &bangumi.CharacterResponse{ID: rc.ID, Name: rc.Name, Type: rc.Type, Images: rc.Images}
			}
			results <- charJob{rc: rc, detail: detail}
		}(rc)
	}
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	imgJobs := make(chan charJob, len(chars))
	go func() {
		defer writerWg.Done()
		defer close(imgJobs)
		tx, _ := s.DB.Begin()
		defer tx.Rollback()
		done := 0
		for job := range results {
			infoboxJSON, _ := json.Marshal(job.detail.Infobox)
			db.UpsertCharacter(tx, job.detail.ID, job.detail.Name, job.detail.Type, job.detail.Summary, job.detail.Gender,
				job.detail.BloodType, job.detail.BirthYear, job.detail.BirthMon, job.detail.BirthDay,
				job.detail.Locked, job.detail.NSFW, "", string(infoboxJSON),
				job.detail.Stat.Comments, job.detail.Stat.Collects)
			db.UpsertSubjectCharacter(tx, subjectID, job.rc.ID, job.rc.Relation)
			for _, actor := range job.rc.Actors {
				db.UpsertPerson(tx, actor.ID, actor.Name, actor.Type, actor.Summary, "", 0, 0, 0, 0, actor.Locked, "", "[]", actor.Career, 0, 0)
				db.UpsertCharacterPerson(tx, job.rc.ID, actor.ID, subjectID)
			}
			done++
			t.Send(fmt.Sprintf(`{"step":"characters","done":%d,"total":%d}`, done, len(chars)))
			imgJobs <- job
		}
		tx.Commit()
	}()
	wg.Wait()
	close(results)
	writerWg.Wait()
	s.dlCharImgs(imgJobs, concurrency)
}

type personJob struct{ rp bangumi.RelatedPerson; detail *bangumi.PersonResponse }

func (s *Service) pipePersons(subjectID int, persons []bangumi.RelatedPerson, concurrency int, t *task.Task) {
	results := make(chan personJob, len(persons))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, rp := range persons {
		wg.Add(1)
		go func(rp bangumi.RelatedPerson) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detail, err := s.Client.GetPerson(rp.ID)
			if err != nil {
				detail = &bangumi.PersonResponse{ID: rp.ID, Name: rp.Name, Type: rp.Type, Career: rp.Career, Images: rp.Images}
			}
			results <- personJob{rp: rp, detail: detail}
		}(rp)
	}
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	imgJobs := make(chan personJob, len(persons))
	go func() {
		defer writerWg.Done()
		defer close(imgJobs)
		tx, _ := s.DB.Begin()
		defer tx.Rollback()
		done := 0
		for job := range results {
			infoboxJSON, _ := json.Marshal(job.detail.Infobox)
			db.UpsertPerson(tx, job.detail.ID, job.detail.Name, job.detail.Type, job.detail.Summary, job.detail.Gender,
				job.detail.BloodType, job.detail.BirthYear, job.detail.BirthMon, job.detail.BirthDay,
				job.detail.Locked, "", string(infoboxJSON), job.detail.Career,
				job.detail.Stat.Comments, job.detail.Stat.Collects)
			db.UpsertSubjectPerson(tx, subjectID, job.rp.ID, job.rp.Relation, job.rp.Eps)
			done++
			t.Send(fmt.Sprintf(`{"step":"persons","done":%d,"total":%d}`, done, len(persons)))
			imgJobs <- job
		}
		tx.Commit()
	}()
	wg.Wait()
	close(results)
	writerWg.Wait()
	s.dlPersonImgs(imgJobs, concurrency)
}

func (s *Service) dlCharImgs(jobs <-chan charJob, concurrency int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for job := range jobs {
		if job.detail.Images.Large == "" {
			continue
		}
		wg.Add(1)
		go func(job charJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if p, err := DownloadImage(job.detail.Images.Large, s.DataDir, job.detail.ID, "character"); err == nil {
				s.DB.Exec(`UPDATE character SET image_path = ? WHERE id = ?`, p, job.detail.ID)
			}
			if p, err := DownloadImage(job.detail.Images.Grid, s.DataDir, job.detail.ID, "character_grid"); err == nil {
				s.DB.Exec(`UPDATE character SET image_grid_path = ? WHERE id = ?`, p, job.detail.ID)
			}
		}(job)
	}
	wg.Wait()
}

func (s *Service) dlPersonImgs(jobs <-chan personJob, concurrency int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for job := range jobs {
		if job.detail.Images.Large == "" {
			continue
		}
		wg.Add(1)
		go func(job personJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if p, err := DownloadImage(job.detail.Images.Large, s.DataDir, job.detail.ID, "person"); err == nil {
				s.DB.Exec(`UPDATE person SET image_path = ? WHERE id = ?`, p, job.detail.ID)
			}
			if p, err := DownloadImage(job.detail.Images.Grid, s.DataDir, job.detail.ID, "person_grid"); err == nil {
				s.DB.Exec(`UPDATE person SET image_grid_path = ? WHERE id = ?`, p, job.detail.ID)
			}
		}(job)
	}
	wg.Wait()
}

func (s *Service) fetchEpisodes(subjectID int) error {
	var all []bangumi.EpisodeResponse
	offset := 0
	for {
		page, err := s.Client.GetEpisodes(subjectID, offset, 100)
		if err != nil {
			return err
		}
		all = append(all, page.Data...)
		if offset+100 >= page.Total {
			break
		}
		offset += 100
	}
	tx, _ := s.DB.Begin()
	defer tx.Rollback()
	for _, ep := range all {
		db.UpsertEpisode(tx, ep.ID, subjectID, ep.Type, ep.Sort, ep.Ep, ep.Name, ep.NameCN, ep.Duration, ep.Airdate, ep.Desc, ep.Disc)
	}
	return tx.Commit()
}

func (s *Service) fetchRelations(subjectID int) error {
	rels, err := s.Client.GetSubjectRelations(subjectID)
	if err != nil {
		return err
	}
	tx, _ := s.DB.Begin()
	defer tx.Rollback()
	for _, r := range rels {
		db.EnsureSubjectStub(tx, r.ID, r.Name, r.NameCN)
		db.UpsertSubjectRelation(tx, subjectID, r.ID, r.Relation)
	}
	return tx.Commit()
}
