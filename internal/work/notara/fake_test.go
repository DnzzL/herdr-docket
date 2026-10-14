package notara

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

// fakeNotara is the slice of Notara's REST /api/v1 the adapter speaks, one
// database deep. Its JSON is copied from a live Notara's answers (2026-10-14,
// the imported Backlog database), not from what the adapter expects: a fake
// that agrees with the code rather than the world is the failure this repo
// already shipped once. What it refuses, it refuses as Notara does — a select
// value that is not one of the field's options is a 422 and writes nothing.
type fakeNotara struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	key      string
	fields   []fakeField
	records  []*fakeRecord
	writes   int // successful writes, so a refused one can be shown to change nothing
	requests []string
}

type fakeField struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Options []string `json:"options"`
}

type fakeRecord struct {
	ID        string
	ShortID   string
	Title     string
	CreatedAt string
	Deleted   bool
	Fields    map[string]any
	Body      string
	Criteria  []fakeCriterion
	Comments  []map[string]any
}

type fakeCriterion struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

const (
	fakeWorkspace = "01M4TVGT0M1H5TEHJHM5GXH0QJ"
	fakeDatabase  = "01M4TVH2VHCM7A7QKP4EG8JR5F"
	fakeKey       = "ntr_test"
)

func newFakeNotara(t *testing.T) *fakeNotara {
	t.Helper()
	f := &fakeNotara{t: t, key: fakeKey, fields: []fakeField{
		{ID: "f-status", Name: "Status", Type: "select", Options: []string{"needs-triage", "ready-for-agent", "in-progress", "ready-for-human", "failed", "blocked", "done", "wontfix"}},
		{ID: "f-agent", Name: "Agent", Type: "select", Options: []string{"thomas", "dev", "reviewer"}},
		{ID: "f-labels", Name: "Labels", Type: "multiSelect", Options: []string{"bug", "critical"}},
		{ID: "f-priority", Name: "Priority", Type: "select", Options: []string{"high", "medium", "low"}},
	}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// source is a Source on this fake, signed in with the right key.
func (f *fakeNotara) source(t *testing.T) *Source {
	t.Helper()
	s, err := New(Config{URL: f.srv.URL, Workspace: fakeWorkspace, Database: fakeDatabase}, testVocabulary, "")
	if err != nil {
		t.Fatal(err)
	}
	s.api.token = func() (string, error) { return fakeKey, nil }
	return s
}

// seed puts a card in the database as an import would have, with no request.
func (f *fakeNotara) seed(r fakeRecord) *fakeRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.add(r)
}

// add is seed for a caller that already holds the lock.
func (f *fakeNotara) add(r fakeRecord) *fakeRecord {
	n := len(f.records) + 1
	if r.ID == "" {
		r.ID = fmt.Sprintf("01REC%04d", n)
	}
	if r.ShortID == "" {
		r.ShortID = fmt.Sprintf("NOT-%d", n)
	}
	if r.CreatedAt == "" {
		r.CreatedAt = fmt.Sprintf("2026-10-13T14:44:%02d.000Z", n)
	}
	if r.Fields == nil {
		r.Fields = map[string]any{}
	}
	rec := r
	f.records = append(f.records, &rec)
	return &rec
}

func (f *fakeNotara) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer "+f.key {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Invalid API key"})
		return
	}
	base := "/api/v1/workspaces/" + fakeWorkspace + "/databases/" + fakeDatabase
	path, ok := strings.CutPrefix(r.URL.Path, base)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Database not found"})
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case len(parts) == 1 && parts[0] == "fields" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, f.fields)
	case len(parts) == 2 && parts[0] == "fields" && r.Method == http.MethodPatch:
		f.patchField(w, r, parts[1])
	case len(parts) == 1 && parts[0] == "records" && r.Method == http.MethodGet:
		f.list(w)
	case len(parts) == 1 && parts[0] == "records" && r.Method == http.MethodPost:
		f.create(w, r)
	case len(parts) >= 2 && parts[0] == "records":
		rec := f.find(parts[1])
		if rec == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "Record not found: " + parts[1]})
			return
		}
		switch {
		case len(parts) == 2 && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, f.document(rec))
		case len(parts) == 2 && r.Method == http.MethodPatch:
			f.patch(w, r, rec)
		case len(parts) == 3 && parts[2] == "comments" && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, orEmpty(rec.Comments))
		case len(parts) == 3 && parts[2] == "comments" && r.Method == http.MethodPost:
			f.comment(w, r, rec)
		case len(parts) == 3 && parts[2] == "criteria" && r.Method == http.MethodPost:
			f.addCriteria(w, r, rec)
		default:
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "Not found"})
		}
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Not found"})
	}
}

func (f *fakeNotara) find(id string) *fakeRecord {
	for _, r := range f.records {
		if (r.ID == id || r.ShortID == id) && !r.Deleted {
			return r
		}
	}
	return nil
}

// list answers as Notara's listDatabaseRecords does: every record, trashed
// ones flagged rather than dropped, fields with no value left out of the map.
func (f *fakeNotara) list(w http.ResponseWriter) {
	out := make([]map[string]any, 0, len(f.records))
	for _, r := range f.records {
		out = append(out, map[string]any{
			"id": r.ID, "shortId": r.ShortID, "databaseId": fakeDatabase,
			"title": r.Title, "description": "", "isDeleted": r.Deleted,
			"createdAt": r.CreatedAt, "fields": r.Fields,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (f *fakeNotara) document(r *fakeRecord) map[string]any {
	return map[string]any{
		"id": r.ID, "shortId": r.ShortID, "databaseId": fakeDatabase,
		"title": r.Title, "description": "", "pageId": "01PAGE" + r.ID,
		"isDeleted": r.Deleted, "createdAt": r.CreatedAt, "deletedAt": nil,
		"fields": r.Fields, "body": r.Body, "criteria": orEmpty(r.Criteria),
	}
}

// validate is Notara's all-or-nothing field check: an unknown field or a
// select value outside its options names the field, and nothing is written.
func (f *fakeNotara) validate(fields map[string]any) (map[string]any, string) {
	out := map[string]any{}
	for name, v := range fields {
		i := slices.IndexFunc(f.fields, func(x fakeField) bool { return x.Name == name || x.ID == name })
		if i < 0 {
			return nil, fmt.Sprintf("Unknown field %q", name)
		}
		fd := f.fields[i]
		// Notara's invalidValue: a select takes one option as a string, and
		// "" is the empty value (it clears); null is not a string.
		if fd.Type == "select" {
			val, ok := v.(string)
			if !ok {
				return nil, fmt.Sprintf("Field %q takes one option, as a string", fd.Name)
			}
			if val != "" && !slices.Contains(fd.Options, val) {
				return nil, fmt.Sprintf("Field %q has no option %q", fd.Name, val)
			}
		}
		out[fd.Name] = v
	}
	return out, ""
}

func apply(r *fakeRecord, fields map[string]any) {
	for k, v := range fields {
		if v == nil || v == "" {
			delete(r.Fields, k)
			continue
		}
		r.Fields[k] = v
	}
}

func (f *fakeNotara) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title    string         `json:"title"`
		Body     string         `json:"body"`
		Fields   map[string]any `json:"fields"`
		Criteria []string       `json:"criteria"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": `Field "title" is required`})
		return
	}
	fields, bad := f.validate(in.Fields)
	if bad != "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": bad})
		return
	}
	rec := f.add(fakeRecord{Title: in.Title, Body: in.Body})
	apply(rec, fields)
	f.writes++
	writeJSON(w, http.StatusCreated, f.document(rec))
}

func (f *fakeNotara) patch(w http.ResponseWriter, r *http.Request, rec *fakeRecord) {
	var in struct {
		Fields map[string]any `json:"fields"`
	}
	if !decode(w, r, &in) {
		return
	}
	fields, bad := f.validate(in.Fields)
	if bad != "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": bad})
		return
	}
	apply(rec, fields)
	f.writes++
	writeJSON(w, http.StatusOK, f.document(rec))
}

func (f *fakeNotara) comment(w http.ResponseWriter, r *http.Request, rec *fakeRecord) {
	var in struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	c := map[string]any{
		"id": fmt.Sprintf("01CMT%d", len(rec.Comments)+1), "pageId": "01PAGE" + rec.ID,
		"authorId": "user-thomas", "body": in.Body, "createdAt": "2026-10-14T09:00:00.000Z",
		"updatedAt": nil, "keyName": "fleet", "mentions": []any{},
	}
	rec.Comments = append(rec.Comments, c)
	f.writes++
	writeJSON(w, http.StatusCreated, c)
}

func (f *fakeNotara) addCriteria(w http.ResponseWriter, r *http.Request, rec *fakeRecord) {
	var in struct {
		Criteria []string `json:"criteria"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Criteria) == 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": `Field "criteria" must name at least one criterion`})
		return
	}
	for _, text := range in.Criteria {
		rec.Criteria = append(rec.Criteria, fakeCriterion{ID: fmt.Sprintf("01BLK%d:0", len(rec.Criteria)), Text: text})
		rec.Body += "\n\n- [ ] " + text
	}
	f.writes++
	writeJSON(w, http.StatusCreated, f.document(rec))
}

func (f *fakeNotara) patchField(w http.ResponseWriter, r *http.Request, id string) {
	var in struct {
		Options []string `json:"options"`
	}
	if !decode(w, r, &in) {
		return
	}
	for i := range f.fields {
		if f.fields[i].ID == id {
			f.fields[i].Options = in.Options
			f.writes++
			writeJSON(w, http.StatusOK, f.fields[i])
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "Field not found: " + id})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid JSON body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func orEmpty[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

func (f *fakeNotara) field(name string) fakeField {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.fields {
		if x.Name == name {
			return x
		}
	}
	f.t.Fatalf("no field %q", name)
	return fakeField{}
}

// get and writeCount read the fake's state for a test, under its lock.
func (f *fakeNotara) get(id string) fakeRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.find(id); r != nil {
		return *r
	}
	f.t.Fatalf("no record %s", id)
	return fakeRecord{}
}

func (f *fakeNotara) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes
}
