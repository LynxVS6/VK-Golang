package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var DatasetPath = "dataset.xml"

type xmlUser struct {
	ID        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

func parseInt(q map[string][]string, key string, def int) (int, error) {
	if v, ok := q[key]; ok && len(v) > 0 && v[0] != "" {
		i, err := strconv.Atoi(v[0])
		if err != nil {
			return 0, err
		}
		return i, nil
	}
	return def, nil
}

func parseOrderBy(q map[string][]string) (int, error) {
	v, err := parseInt(q, "order_by", OrderByAsIs)
	if err != nil {
		return 0, err
	}
	if v != OrderByAsc && v != OrderByDesc && v != OrderByAsIs {
		return 0, errors.New("bad order_by")
	}
	return v, nil
}

func parseOrderField(q map[string][]string) (string, error) {
	f := ""
	if v, ok := q["order_field"]; ok && len(v) > 0 {
		f = v[0]
	}
	if f == "" {
		return "Name", nil
	}
	switch strings.ToLower(f) {
	case "id":
		return "Id", nil
	case "age":
		return "Age", nil
	case "name":
		return "Name", nil
	default:
		return "", errors.New(ErrorBadOrderField)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readUsersFromXML(path string) ([]User, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := xml.NewDecoder(f)
	var out []User

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		var xu xmlUser
		if err := dec.DecodeElement(&xu, &se); err != nil {
			return nil, err
		}
		out = append(out, User{
			ID:     xu.ID,
			Name:   strings.TrimSpace(xu.FirstName + " " + xu.LastName),
			Age:    xu.Age,
			About:  xu.About,
			Gender: xu.Gender,
		})
	}
	return out, nil
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("AccessToken") == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()

	limit, err := parseInt(q, "limit", 0)
	if err != nil || limit < 0 {
		writeJSON(w, http.StatusBadRequest, SearchErrorResponse{Error: "bad limit"})
		return
	}
	offset, err := parseInt(q, "offset", 0)
	if err != nil || offset < 0 {
		writeJSON(w, http.StatusBadRequest, SearchErrorResponse{Error: "bad offset"})
		return
	}
	orderBy, err := parseOrderBy(q)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, SearchErrorResponse{Error: err.Error()})
		return
	}
	orderField, err := parseOrderField(q)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, SearchErrorResponse{Error: ErrorBadOrderField})
		return
	}
	query := strings.ToLower(q.Get("query"))

	if q.Get("slow") == "1" {
		time.Sleep(2 * time.Second)
	}

	users, err := readUsersFromXML(DatasetPath)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// фильтрация
	var filtered []User
	if query == "" {
		filtered = users
	} else {
		for _, u := range users {
			if strings.Contains(strings.ToLower(u.Name), query) ||
				strings.Contains(strings.ToLower(u.About), query) {
				filtered = append(filtered, u)
			}
		}
	}

	// сортировка
	if orderBy != OrderByAsIs {
		sort.Slice(filtered, func(i, j int) bool {
			var less bool
			switch orderField {
			case "Id":
				less = filtered[i].ID < filtered[j].ID
			case "Age":
				less = filtered[i].Age < filtered[j].Age
			default: // "Name"
				less = filtered[i].Name < filtered[j].Name
			}
			if orderBy == OrderByAsc {
				return less
			}
			return !less
		})
	}

	// пагинация
	if offset > len(filtered) {
		writeJSON(w, http.StatusOK, []User{})
		return
	}
	end := offset + limit
	if limit == 0 || end > len(filtered) {
		end = len(filtered)
	}
	writeJSON(w, http.StatusOK, filtered[offset:end])
}
