package basecamp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// fakeServer is an in-memory Basecamp: enough of the API for the adapter to be
// driven through it exactly as the fleet drives it for real. It exists so the
// shared contract suite can run against a backend it can empty between
// subtests, and so the HTTP layer is exercised rather than stubbed out — a
// contract proved against a fake client would only prove the fake.
type fakeServer struct {
	mu       sync.Mutex
	seq      int
	lists    map[string][]*todoRecord
	byID     map[string]*todoRecord
	comments map[string][]comment
}

// newFakeServer is a Basecamp with the given to-do lists, all of them empty.
func newFakeServer(lists ...string) *fakeServer {
	f := &fakeServer{
		lists:    make(map[string][]*todoRecord, len(lists)),
		byID:     map[string]*todoRecord{},
		comments: map[string][]comment{},
	}
	for _, l := range lists {
		f.lists[l] = []*todoRecord{}
	}
	return f
}

// todoRecord is a to-do as Basecamp's API would return it. It carries the same
// JSON tags as the adapter's read struct on purpose: what this serves is what
// the adapter would get from the real thing.
type todoRecord struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Completed bool   `json:"completed"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Position  int    `json:"position"`
	CreatedAt string `json:"created_at"`
	Steps     []step `json:"steps"`
	Parent    ref    `json:"parent"`
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	seg := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case len(seg) == 3 && seg[0] == "todolists" && seg[2] == "todos.json":
		f.todos(w, r, seg[1])
	case len(seg) == 3 && seg[0] == "todos" && seg[2] == "completion.json":
		f.complete(w, seg[1])
	case len(seg) == 2 && seg[0] == "todos" && strings.HasSuffix(seg[1], ".json"):
		f.oneTodo(w, strings.TrimSuffix(seg[1], ".json"))
	case len(seg) == 3 && seg[0] == "recordings" && seg[2] == "comments.json":
		f.commentsFor(w, r, seg[1])
	default:
		f.notFound(w)
	}
}

func (f *fakeServer) todos(w http.ResponseWriter, r *http.Request, list string) {
	todos, ok := f.lists[list]
	if !ok {
		f.notFound(w)
		return
	}
	if r.Method != http.MethodPost {
		f.write(w, todos)
		return
	}
	var in content
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// The list a to-do lands in is the whole of Basecamp's routing, so the
	// fake has to record parent the way the real API does.
	listID, err := strconv.Atoi(list)
	if err != nil {
		http.Error(w, "the fake's list ids are numeric", http.StatusInternalServerError)
		return
	}
	f.seq++
	t := &todoRecord{
		ID:        f.seq,
		Status:    "active",
		Title:     in.Title,
		Content:   in.Content,
		CreatedAt: "2026-01-01T09:00:00.000Z",
		Parent:    ref{ID: listID, Type: "Todolist"},
	}
	f.lists[list] = append(f.lists[list], t)
	f.byID[strconv.Itoa(t.ID)] = t
	w.WriteHeader(http.StatusCreated)
	f.write(w, t)
}

func (f *fakeServer) oneTodo(w http.ResponseWriter, id string) {
	t, ok := f.byID[id]
	if !ok {
		f.notFound(w)
		return
	}
	f.write(w, t)
}

// complete answers the way Basecamp does: 204, with nothing to say.
func (f *fakeServer) complete(w http.ResponseWriter, id string) {
	t, ok := f.byID[id]
	if !ok {
		f.notFound(w)
		return
	}
	t.Completed, t.Status = true, "completed"
	w.WriteHeader(http.StatusNoContent)
}

func (f *fakeServer) commentsFor(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := f.byID[id]; !ok {
		f.notFound(w)
		return
	}
	if r.Method != http.MethodPost {
		f.write(w, f.comments[id])
		return
	}
	var in content
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.seq++
	c := comment{ID: f.seq, Content: in.Content, CreatedAt: "2026-01-01T10:00:00.000Z"}
	f.comments[id] = append(f.comments[id], c)
	w.WriteHeader(http.StatusCreated)
	f.write(w, c)
}

func (f *fakeServer) write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (f *fakeServer) notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error":"Not found"}`))
}
