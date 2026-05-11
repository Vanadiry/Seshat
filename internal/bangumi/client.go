package bangumi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const baseURL = "https://api.bgm.tv"

type Client struct {
	http *http.Client
	ua   string
}

func NewClient(ua string) *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}, ua: ua}
}

func (c *Client) get(url string, v any) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found (404): %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// getWithRetry 带指数退避重试，最多 maxRetries 次，不重试 404。
func (c *Client) getWithRetry(url string, v any, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = c.get(url, v)
		if err == nil {
			return nil
		}
		if err.Error()[:13] == "not found (40" {
			return err
		}
		if i < maxRetries-1 {
			time.Sleep(time.Duration(1<<uint(i)) * time.Second)
		}
	}
	return err
}

func (c *Client) GetSubject(id int) (*SubjectResponse, error) {
	var v SubjectResponse
	return &v, c.get(fmt.Sprintf("%s/v0/subjects/%d", baseURL, id), &v)
}

func (c *Client) GetSubjectCharacters(id int) ([]RelatedCharacter, error) {
	var v []RelatedCharacter
	return v, c.get(fmt.Sprintf("%s/v0/subjects/%d/characters", baseURL, id), &v)
}

func (c *Client) GetSubjectPersons(id int) ([]RelatedPerson, error) {
	var v []RelatedPerson
	return v, c.get(fmt.Sprintf("%s/v0/subjects/%d/persons", baseURL, id), &v)
}

func (c *Client) GetSubjectRelations(id int) ([]SubjectRelationResponse, error) {
	var v []SubjectRelationResponse
	return v, c.get(fmt.Sprintf("%s/v0/subjects/%d/subjects", baseURL, id), &v)
}

func (c *Client) GetCharacter(id int) (*CharacterResponse, error) {
	var v CharacterResponse
	return &v, c.getWithRetry(fmt.Sprintf("%s/v0/characters/%d", baseURL, id), &v, 3)
}

func (c *Client) GetPerson(id int) (*PersonResponse, error) {
	var v PersonResponse
	return &v, c.getWithRetry(fmt.Sprintf("%s/v0/persons/%d", baseURL, id), &v, 3)
}

func (c *Client) GetEpisodes(subjectID, offset, limit int) (*PagedEpisodes, error) {
	var v PagedEpisodes
	return &v, c.get(fmt.Sprintf("%s/v0/episodes?subject_id=%d&offset=%d&limit=%d", baseURL, subjectID, offset, limit), &v)
}
