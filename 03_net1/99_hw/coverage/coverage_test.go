package main

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type tXMLUser struct {
	ID        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

func parseAllUsersFromDataset(t *testing.T) []User {
	t.Helper()

	f, err := os.Open("dataset.xml")
	if err != nil {
		t.Fatalf("open dataset: %v", err)
	}
	defer f.Close()

	dec := xml.NewDecoder(f)
	var res []User
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode token: %v", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		var r tXMLUser
		if err := dec.DecodeElement(&r, &se); err != nil {
			t.Fatalf("decode elem: %v", err)
		}
		res = append(res, User{
			ID:     r.ID,
			Name:   strings.TrimSpace(r.FirstName + " " + r.LastName),
			Age:    r.Age,
			About:  r.About,
			Gender: r.Gender,
		})
	}
	return res
}

func TestFindUsers_OK_BasicAndPaging(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	c := &SearchClient{AccessToken: "ok", URL: ts.URL}

	resp, err := c.FindUsers(SearchRequest{
		Limit:      2, // клиент увеличит до 3 для NextPage
		Offset:     0,
		Query:      "",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !resp.NextPage {
		t.Fatalf("NextPage expected true")
	}
	if len(resp.Users) != 2 {
		t.Fatalf("want 2 users, got %d", len(resp.Users))
	}
	if !(resp.Users[0].ID < resp.Users[1].ID) {
		t.Fatalf("ascending by ID expected")
	}
}

func TestFindUsers_QueryAndOrderDesc_MaxByAge(t *testing.T) {
	all := parseAllUsersFromDataset(t)

	var filtered []User
	for _, u := range all {
		if strings.Contains(strings.ToLower(u.Name), "on") ||
			strings.Contains(strings.ToLower(u.About), "on") {
			filtered = append(filtered, u)
		}
	}
	if len(filtered) == 0 {
		t.Skip("dataset has no matches for 'on'")
	}
	max := filtered[0]
	for _, u := range filtered[1:] {
		if u.Age > max.Age {
			max = u
		}
	}

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	c := &SearchClient{AccessToken: "x", URL: ts.URL}

	resp, err := c.FindUsers(SearchRequest{
		Limit:      1,
		Offset:     0,
		Query:      "on",
		OrderField: "age",
		OrderBy:    OrderByDesc,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resp.Users) != 1 {
		t.Fatalf("want 1 user, got %d", len(resp.Users))
	}
	if resp.Users[0].ID != max.ID || resp.Users[0].Age != max.Age {
		t.Fatalf("wrong user chosen: got %+v, want %+v", resp.Users[0], max)
	}
}

func TestFindUsers_DefaultOrderFieldAndAsIs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	c := &SearchClient{AccessToken: "tt", URL: ts.URL}

	resp, err := c.FindUsers(SearchRequest{
		Limit:      3,
		Offset:     0,
		OrderField: "", // по умолчанию Name
		OrderBy:    OrderByAsIs,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resp.Users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(resp.Users))
	}
}

func TestFindUsers_ClientValidation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	c := &SearchClient{AccessToken: "x", URL: ts.URL}

	_, err := c.FindUsers(SearchRequest{Limit: -1})
	if err == nil || !strings.Contains(err.Error(), "limit must be > 0") {
		t.Fatalf("want limit validation error, got %v", err)
	}
	_, err = c.FindUsers(SearchRequest{Limit: 1, Offset: -5})
	if err == nil || !strings.Contains(err.Error(), "offset must be > 0") {
		t.Fatalf("want offset validation error, got %v", err)
	}
}

func TestFindUsers_Server400_OrderFieldInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	c := &SearchClient{AccessToken: "x", URL: ts.URL}

	_, err := c.FindUsers(SearchRequest{
		Limit:      1,
		OrderField: "unknown",
	})
	if err == nil || !strings.Contains(err.Error(), "OrderFeld unknown invalid") {
		t.Fatalf("want 'OrderFeld unknown invalid', got %v", err)
	}
}

func TestFindUsers_Server400_UnknownBadRequest(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"Error":"bad order_by"}`))
	}))
	defer s.Close()
	c := &SearchClient{AccessToken: "x", URL: s.URL}

	_, err := c.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "unknown bad request error: bad order_by") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestFindUsers_Server401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	c := &SearchClient{AccessToken: "", URL: ts.URL}

	_, err := c.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "bad AccessToken") {
		t.Fatalf("want bad AccessToken, got %v", err)
	}
}

func TestFindUsers_Server500(t *testing.T) {
	old := DatasetPath
	DatasetPath = "no_such_file.xml"
	defer func() { DatasetPath = old }()

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	c := &SearchClient{AccessToken: "ok", URL: ts.URL}

	_, err := c.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "SearchServer fatal error") {
		t.Fatalf("want 500 mapping, got %v", err)
	}
}

func TestFindUsers_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := &SearchClient{AccessToken: "ok", URL: ts.URL}
	_, err := c.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "timeout for") {
		t.Fatalf("want timeout error, got %v", err)
	}
}

func TestFindUsers_BadErrorJSON(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`not json`))
	}))
	defer s.Close()
	c := &SearchClient{AccessToken: "t", URL: s.URL}
	_, err := c.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "cant unpack error json") {
		t.Fatalf("want bad error json, got %v", err)
	}
}

func TestFindUsers_BadResultJSON(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{`)) // битый JSON
	}))
	defer s.Close()
	c := &SearchClient{AccessToken: "t", URL: s.URL}
	_, err := c.FindUsers(SearchRequest{Limit: 1})
	if err == nil || !strings.Contains(err.Error(), "cant unpack result json") {
		t.Fatalf("want bad result json, got %v", err)
	}
}

func TestSearchServer_BadOrderByParam(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"?order_by=42", nil)
	req.Header.Set("AccessToken", "ok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestSearchServer_SlowQueryHook(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	start := time.Now()
	req, _ := http.NewRequest("GET", ts.URL+"?slow=1&limit=1", nil)
	req.Header.Set("AccessToken", "ok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()
	if time.Since(start) < time.Second {
		t.Fatalf("expected slow response")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}
