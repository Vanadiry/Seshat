package query

import (
	"database/sql"
	"strings"

	"github.com/vanadiry/seshat/internal/model"
)

type Queries struct{ DB *sql.DB }

func New(db *sql.DB) *Queries { return &Queries{DB: db} }

func (q *Queries) ListSubjects(search, tag, platform, year string, page, limit int) ([]model.Subject, int, error) {
	if limit < 1 {
		limit = 30
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	var conds []string
	var args []any
	if search != "" {
		conds = append(conds, "(name LIKE ? OR name_cn LIKE ? OR summary LIKE ?)")
		a := "%" + search + "%"
		args = append(args, a, a, a)
	}
	if tag != "" {
		conds = append(conds, "id IN (SELECT subject_id FROM subject_tag WHERE tag_name = ?)")
		args = append(args, tag)
	}
	if platform != "" {
		conds = append(conds, "platform = ?")
		args = append(args, platform)
	}
	if year != "" {
		conds = append(conds, "date LIKE ?")
		args = append(args, year+"%")
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := q.DB.QueryRow("SELECT COUNT(*) FROM subject"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := q.DB.Query("SELECT id, type, name, name_cn, summary, date, platform, eps, total_episodes, volumes, series, locked, nsfw, score, rank, rating_total, wish_count, collect_count, doing_count, on_hold_count, dropped_count, image_path, infobox, created_at, updated_at FROM subject"+where+" ORDER BY id LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanSubjects(rows), total, nil
}

func (q *Queries) GetSubject(id int) (*model.Subject, error) {
	row := q.DB.QueryRow("SELECT id, type, name, name_cn, summary, date, platform, eps, total_episodes, volumes, series, locked, nsfw, score, rank, rating_total, wish_count, collect_count, doing_count, on_hold_count, dropped_count, image_path, infobox, created_at, updated_at FROM subject WHERE id = ?", id)
	return scanSubject(row)
}

func (q *Queries) GetSubjectCharacters(subjectID int) ([]model.SubjectCharacter, error) {
	rows, err := q.DB.Query(`SELECT sc.subject_id, sc.character_id, sc.relation, c.id, c.name, c.type, c.summary, c.gender, c.image_path FROM subject_character sc JOIN character c ON c.id = sc.character_id WHERE sc.subject_id = ? ORDER BY c.id`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SubjectCharacter
	for rows.Next() {
		var sc model.SubjectCharacter
		var c model.Character
		var gender sql.NullString
		if err := rows.Scan(&sc.SubjectID, &sc.CharacterID, &sc.Relation, &c.ID, &c.Name, &c.Type, &c.Summary, &gender, &c.ImagePath); err != nil {
			return nil, err
		}
		c.Gender = gender.String
		sc.Character = &c
		out = append(out, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	actorRows, err := q.DB.Query(`SELECT cp.character_id, p.id, p.name FROM character_person cp JOIN person p ON p.id = cp.person_id WHERE cp.subject_id = ?`, subjectID)
	if err != nil {
		return out, nil
	}
	defer actorRows.Close()
	actorsByChar := make(map[int][]model.SubjectCharacterActor)
	for actorRows.Next() {
		var charID, personID int
		var name string
		if err := actorRows.Scan(&charID, &personID, &name); err != nil {
			continue
		}
		actorsByChar[charID] = append(actorsByChar[charID], model.SubjectCharacterActor{PersonID: personID, Name: name})
	}
	for i := range out {
		out[i].Actors = actorsByChar[out[i].CharacterID]
	}
	return out, nil
}

func (q *Queries) GetSubjectPersons(subjectID int) ([]model.SubjectPerson, error) {
	rows, err := q.DB.Query(`SELECT sp.subject_id, sp.person_id, sp.relation, sp.eps, p.id, p.name, p.type, p.career, p.image_path FROM subject_person sp JOIN person p ON p.id = sp.person_id WHERE sp.subject_id = ? ORDER BY p.id`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SubjectPerson
	for rows.Next() {
		var sp model.SubjectPerson
		var p model.Person
		if err := rows.Scan(&sp.SubjectID, &sp.PersonID, &sp.Relation, &sp.Eps, &p.ID, &p.Name, &p.Type, &p.Career, &p.ImagePath); err != nil {
			return nil, err
		}
		sp.Person = &p
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (q *Queries) GetSubjectTags(subjectID int) ([]model.Tag, error) {
	rows, err := q.DB.Query("SELECT tag_name, count FROM subject_tag WHERE subject_id = ? ORDER BY count DESC", subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Tag
	for rows.Next() {
		var t model.Tag
		rows.Scan(&t.Name, &t.Count)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (q *Queries) GetSubjectEpisodes(subjectID int) ([]model.Episode, error) {
	rows, err := q.DB.Query(`SELECT id, subject_id, type, sort, ep, name, name_cn, duration, airdate, "desc", disc FROM episode WHERE subject_id = ? ORDER BY sort`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEpisodes(rows)
}

func (q *Queries) GetSubjectRelations(subjectID int) ([]model.SubjectRelation, error) {
	rows, err := q.DB.Query("SELECT subject_id, related_id, relation FROM subject_relation WHERE subject_id = ?", subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SubjectRelation
	for rows.Next() {
		var r model.SubjectRelation
		rows.Scan(&r.SubjectID, &r.RelatedID, &r.Relation)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) GetCharacter(id int) (*model.Character, error) {
	row := q.DB.QueryRow("SELECT id, name, type, summary, gender, blood_type, birth_year, birth_mon, birth_day, locked, nsfw, image_path, infobox, comment_count, collect_count, created_at, updated_at FROM character WHERE id = ?", id)
	return scanCharacter(row)
}

func (q *Queries) GetCharacterSubjects(charID int) ([]model.SubjectCharacter, error) {
	rows, err := q.DB.Query(`SELECT sc.subject_id, sc.character_id, sc.relation, s.id, s.name, s.name_cn, s.image_path FROM subject_character sc JOIN subject s ON s.id = sc.subject_id WHERE sc.character_id = ?`, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SubjectCharacter
	for rows.Next() {
		var sc model.SubjectCharacter
		var s model.Subject
		rows.Scan(&sc.SubjectID, &sc.CharacterID, &sc.Relation, &s.ID, &s.Name, &s.NameCN, &s.ImagePath)
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (q *Queries) GetCharacterPersons(charID int) ([]model.SubjectPerson, error) {
	rows, err := q.DB.Query(`SELECT cp.subject_id, cp.person_id, p.id, p.name, p.career FROM character_person cp JOIN person p ON p.id = cp.person_id WHERE cp.character_id = ?`, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SubjectPerson
	for rows.Next() {
		var sp model.SubjectPerson
		var p model.Person
		rows.Scan(&sp.SubjectID, &sp.PersonID, &p.ID, &p.Name, &p.Career)
		sp.Person = &p
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (q *Queries) GetPerson(id int) (*model.Person, error) {
	row := q.DB.QueryRow("SELECT id, name, type, summary, gender, blood_type, birth_year, birth_mon, birth_day, locked, image_path, infobox, career, comment_count, collect_count, created_at, updated_at FROM person WHERE id = ?", id)
	return scanPerson(row)
}

func (q *Queries) GetPersonSubjects(personID int) ([]model.SubjectPerson, error) {
	rows, err := q.DB.Query(`SELECT sp.subject_id, sp.person_id, sp.relation, sp.eps, s.id, s.name, s.name_cn, s.image_path FROM subject_person sp JOIN subject s ON s.id = sp.subject_id WHERE sp.person_id = ? ORDER BY s.date DESC`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SubjectPerson
	for rows.Next() {
		var sp model.SubjectPerson
		var s model.Subject
		rows.Scan(&sp.SubjectID, &sp.PersonID, &sp.Relation, &sp.Eps, &s.ID, &s.Name, &s.NameCN, &s.ImagePath)
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (q *Queries) GetPersonCharacters(personID int) ([]model.SubjectCharacter, error) {
	rows, err := q.DB.Query(`SELECT cp.subject_id, cp.character_id, c.id, c.name FROM character_person cp JOIN character c ON c.id = cp.character_id WHERE cp.person_id = ? ORDER BY cp.subject_id`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SubjectCharacter
	for rows.Next() {
		var sc model.SubjectCharacter
		var c model.Character
		rows.Scan(&sc.SubjectID, &sc.CharacterID, &c.ID, &c.Name)
		sc.Character = &c
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (q *Queries) GetEpisode(id int) (*model.Episode, error) {
	return scanEpisode(q.DB.QueryRow(`SELECT id, subject_id, type, sort, ep, name, name_cn, duration, airdate, "desc", disc FROM episode WHERE id = ?`, id))
}

func (q *Queries) ListTags() ([]model.Tag, error) {
	rows, err := q.DB.Query("SELECT t.name, (SELECT COUNT(*) FROM subject_tag WHERE tag_name = t.name) as cnt FROM tag t ORDER BY cnt DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Tag
	for rows.Next() {
		var t model.Tag
		rows.Scan(&t.Name, &t.Count)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (q *Queries) GetTagSubjects(tagName string, page, limit int) ([]model.Subject, int, error) {
	if limit < 1 {
		limit = 30
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	var total int
	q.DB.QueryRow("SELECT COUNT(*) FROM subject_tag WHERE tag_name = ?", tagName).Scan(&total)
	rows, err := q.DB.Query("SELECT s.id, s.type, s.name, s.name_cn, s.summary, s.date, s.platform, s.eps, s.total_episodes, s.volumes, s.series, s.locked, s.nsfw, s.score, s.rank, s.rating_total, s.wish_count, s.collect_count, s.doing_count, s.on_hold_count, s.dropped_count, s.image_path, s.infobox, s.created_at, s.updated_at FROM subject s JOIN subject_tag st ON st.subject_id = s.id WHERE st.tag_name = ? ORDER BY s.id LIMIT ? OFFSET ?", tagName, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanSubjects(rows), total, nil
}

func scanSubject(row interface{ Scan(...any) error }) (*model.Subject, error) {
	var s model.Subject
	var series, locked, nsfw int
	var score sql.NullFloat64
	var date sql.NullString
	err := row.Scan(&s.ID, &s.Type, &s.Name, &s.NameCN, &s.Summary, &date, &s.Platform, &s.Eps, &s.TotalEpisodes, &s.Volumes, &series, &locked, &nsfw, &score, &s.Rank, &s.RatingTotal, &s.WishCount, &s.CollectCount, &s.DoingCount, &s.OnHoldCount, &s.DroppedCount, &s.ImagePath, &s.Infobox, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.Date = date.String
	s.Series = series == 1
	s.Locked = locked == 1
	s.NSFW = nsfw == 1
	if score.Valid {
		s.Score = score.Float64
	}
	return &s, nil
}

func scanSubjects(rows *sql.Rows) []model.Subject {
	var out []model.Subject
	for rows.Next() {
		var s model.Subject
		var series, locked, nsfw int
		var score sql.NullFloat64
		var date sql.NullString
		rows.Scan(&s.ID, &s.Type, &s.Name, &s.NameCN, &s.Summary, &date, &s.Platform, &s.Eps, &s.TotalEpisodes, &s.Volumes, &series, &locked, &nsfw, &score, &s.Rank, &s.RatingTotal, &s.WishCount, &s.CollectCount, &s.DoingCount, &s.OnHoldCount, &s.DroppedCount, &s.ImagePath, &s.Infobox, &s.CreatedAt, &s.UpdatedAt)
		s.Date = date.String
		s.Series, s.Locked, s.NSFW = series == 1, locked == 1, nsfw == 1
		if score.Valid {
			s.Score = score.Float64
		}
		out = append(out, s)
	}
	return out
}

func scanCharacter(row interface{ Scan(...any) error }) (*model.Character, error) {
	var c model.Character
	var gender sql.NullString
	var bt, by, bm, bd sql.NullInt64
	var locked, nsfw int
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.Summary, &gender, &bt, &by, &bm, &bd, &locked, &nsfw, &c.ImagePath, &c.Infobox, &c.CommentCount, &c.CollectCount, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Gender = gender.String
	c.BloodType, c.BirthYear, c.BirthMon, c.BirthDay = int(bt.Int64), int(by.Int64), int(bm.Int64), int(bd.Int64)
	c.Locked, c.NSFW = locked == 1, nsfw == 1
	return &c, nil
}

func scanPerson(row interface{ Scan(...any) error }) (*model.Person, error) {
	var p model.Person
	var gender sql.NullString
	var bt, by, bm, bd sql.NullInt64
	var locked int
	err := row.Scan(&p.ID, &p.Name, &p.Type, &p.Summary, &gender, &bt, &by, &bm, &bd, &locked, &p.ImagePath, &p.Infobox, &p.Career, &p.CommentCount, &p.CollectCount, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.Gender = gender.String
	p.BloodType, p.BirthYear, p.BirthMon, p.BirthDay = int(bt.Int64), int(by.Int64), int(bm.Int64), int(bd.Int64)
	p.Locked = locked == 1
	return &p, nil
}

func scanEpisode(row interface{ Scan(...any) error }) (*model.Episode, error) {
	var ep model.Episode
	var airdate sql.NullString
	var epFloat sql.NullFloat64
	err := row.Scan(&ep.ID, &ep.SubjectID, &ep.Type, &ep.Sort, &epFloat, &ep.Name, &ep.NameCN, &ep.Duration, &airdate, &ep.Desc, &ep.Disc)
	if err != nil {
		return nil, err
	}
	if airdate.Valid {
		ep.Airdate = airdate.String
	}
	if epFloat.Valid {
		ep.Ep = epFloat.Float64
	}
	return &ep, nil
}

func scanEpisodes(rows *sql.Rows) ([]model.Episode, error) {
	var out []model.Episode
	for rows.Next() {
		ep, err := scanEpisode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ep)
	}
	return out, rows.Err()
}
