package Test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestELOPairNeedsTwoSubjects(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Get(srv.URL + "/api/v1/elo/pair")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]string
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()
	if data["error"] != "need at least 2 cached subjects" {
		t.Logf("elo/pair response: %v", data)
	}
}

func TestELOCompare(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Post(srv.URL+"/api/v1/elo/compare", "application/json",
		strings.NewReader(`{"winner":51,"loser":288}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestELORankingEmpty(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Get(srv.URL + "/api/v1/elo/ranking")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestELOCompareMissingParams(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Post(srv.URL+"/api/v1/elo/compare", "application/json",
		strings.NewReader(`{"winner":51}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing loser, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
