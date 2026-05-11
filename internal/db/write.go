package db

import (
	"database/sql"
	"encoding/json"
)

func DeleteSubjectCascade(tx *sql.Tx, subjectID int) error {
	for _, t := range []string{"subject_tag", "subject_character", "subject_person", "character_person", "subject_relation", "episode"} {
		if _, err := tx.Exec("DELETE FROM "+t+" WHERE subject_id = ?", subjectID); err != nil {
			return err
		}
	}
	return nil
}

func UpsertSubject(tx *sql.Tx, id int, name, nameCN, summary, date, platform string,
	eps, totalEpisodes, volumes int, series, locked, nsfw bool,
	score float64, rank, ratingTotal int,
	wish, collect, doing, onHold, dropped int,
	imagePath, infobox string,
) error {
	_, err := tx.Exec(`INSERT OR REPLACE INTO subject
		(id, type, name, name_cn, summary, date, platform, eps, total_episodes, volumes, series, locked, nsfw,
		 score, rank, rating_total, wish_count, collect_count, doing_count, on_hold_count, dropped_count,
		 image_path, image_grid_path, infobox, updated_at)
		VALUES (?1,2,?2,?3,?4,?5,?6,?7,?8,?9,?10,?11,?12,?13,?14,?15,?16,?17,?18,?19,?20,?21,'',?22,datetime('now'))`,
		id, name, nameCN, summary, date, platform, eps, totalEpisodes, volumes, boolInt(series), boolInt(locked), boolInt(nsfw),
		score, rank, ratingTotal, wish, collect, doing, onHold, dropped, imagePath, infobox)
	return err
}

func UpsertTag(tx *sql.Tx, name string) error {
	_, err := tx.Exec("INSERT OR IGNORE INTO tag (name) VALUES (?)", name)
	return err
}

func UpsertSubjectTag(tx *sql.Tx, subjectID int, tagName string, count int) error {
	_, err := tx.Exec("INSERT OR REPLACE INTO subject_tag (subject_id, tag_name, count) VALUES (?,?,?)", subjectID, tagName, count)
	return err
}

func UpsertCharacter(tx *sql.Tx, id int, name string, ctype int, summary, gender string,
	bloodType, birthYear, birthMon, birthDay int, locked, nsfw bool,
	imagePath, infobox string, commentCount, collectCount int,
) error {
	_, err := tx.Exec(`INSERT OR REPLACE INTO character
		(id, name, type, summary, gender, blood_type, birth_year, birth_mon, birth_day,
		 locked, nsfw, image_path, image_grid_path, infobox, comment_count, collect_count, updated_at)
		VALUES (?1,?2,?3,?4,?5,NULLIF(?6,0),NULLIF(?7,0),NULLIF(?8,0),NULLIF(?9,0),
		 ?10,?11,?12,?13,?14,?15,?16,datetime('now'))`,
		id, name, ctype, summary, gender, bloodType, birthYear, birthMon, birthDay,
		boolInt(locked), boolInt(nsfw), imagePath, "", infobox, commentCount, collectCount)
	return err
}

func UpsertPerson(tx *sql.Tx, id int, name string, ptype int, summary, gender string,
	bloodType, birthYear, birthMon, birthDay int, locked bool,
	imagePath, infobox string, career []string, commentCount, collectCount int,
) error {
	cj, _ := json.Marshal(career)
	_, err := tx.Exec(`INSERT OR REPLACE INTO person
		(id, name, type, summary, gender, blood_type, birth_year, birth_mon, birth_day,
		 locked, image_path, image_grid_path, infobox, career, comment_count, collect_count, updated_at)
		VALUES (?1,?2,?3,?4,?5,NULLIF(?6,0),NULLIF(?7,0),NULLIF(?8,0),NULLIF(?9,0),
		 ?10,?11,?12,?13,?14,?15,?16,datetime('now'))`,
		id, name, ptype, summary, gender, bloodType, birthYear, birthMon, birthDay,
		boolInt(locked), imagePath, "", infobox, string(cj), commentCount, collectCount)
	return err
}

func UpsertSubjectCharacter(tx *sql.Tx, subjectID, characterID int, relation string) error {
	_, err := tx.Exec("INSERT OR REPLACE INTO subject_character (subject_id, character_id, relation) VALUES (?,?,?)", subjectID, characterID, relation)
	return err
}

func UpsertSubjectPerson(tx *sql.Tx, subjectID, personID int, relation, eps string) error {
	_, err := tx.Exec("INSERT OR REPLACE INTO subject_person (subject_id, person_id, relation, eps) VALUES (?,?,?,?)", subjectID, personID, relation, eps)
	return err
}

func UpsertCharacterPerson(tx *sql.Tx, characterID, personID, subjectID int) error {
	_, err := tx.Exec("INSERT OR REPLACE INTO character_person (character_id, person_id, subject_id) VALUES (?,?,?)", characterID, personID, subjectID)
	return err
}

func UpsertSubjectRelation(tx *sql.Tx, subjectID, relatedID int, relation string) error {
	_, err := tx.Exec("INSERT OR REPLACE INTO subject_relation (subject_id, related_id, relation) VALUES (?,?,?)", subjectID, relatedID, relation)
	return err
}

func UpsertEpisode(tx *sql.Tx, id, subjectID, etype int, sort, ep float64,
	name, nameCN, duration, airdate, desc string, disc int,
) error {
	_, err := tx.Exec(`INSERT OR REPLACE INTO episode
		(id, subject_id, type, sort, ep, name, name_cn, duration, airdate, "desc", disc)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, id, subjectID, etype, sort, ep, name, nameCN, duration, airdate, desc, disc)
	return err
}

func EnsureSubjectStub(tx *sql.Tx, id int, name, nameCN string) error {
	_, err := tx.Exec("INSERT OR IGNORE INTO subject (id, type, name, name_cn) VALUES (?,2,?,?)", id, name, nameCN)
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
