package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Task struct {
	Id        string    `json:"Id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TaskRepo interface {
	Create(title string) (Task, error)
	Get(Id string) (Task, bool)
	List(done bool) []Task
	SetDone(Id string, done bool) (Task, error)
}

type Clock interface{ Now() time.Time }

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalIdTitle = errors.New("invalId title")
)

type inMemoryTaskRepo struct {
	mu    sync.RWMutex
	clock Clock
	tasks map[string]Task
}

var nextTaskId atomic.Uint64

func NewInMemoryTaskRepo(clock Clock) TaskRepo {
	return &inMemoryTaskRepo{
		clock: clock,
		tasks: make(map[string]Task),
	}
}

func (r *inMemoryTaskRepo) Create(title string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrInvalIdTitle
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	task := Task{
		Id:        strconv.FormatUint(nextTaskId.Add(1), 10),
		Title:     title,
		UpdatedAt: r.clock.Now(),
	}

	r.tasks[task.Id] = task
	return task, nil
}

func (r *inMemoryTaskRepo) Get(Id string) (Task, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, ok := r.tasks[Id]
	return task, ok
}

func (r *inMemoryTaskRepo) List(done bool) []Task {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tasks := make([]Task, 0)
	for _, task := range r.tasks {
		if task.Done == done {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

func (r *inMemoryTaskRepo) SetDone(Id string, done bool) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[Id]
	if !ok {
		return Task{}, ErrNotFound
	}

	if task.Done == done {
		return task, nil
	}

	task.Done = done
	task.UpdatedAt = r.clock.Now()
	r.tasks[Id] = task
	return task, nil
}

type httpHandler struct{ repo TaskRepo }

func NewHTTPHandler(repo TaskRepo) http.Handler {
	return &httpHandler{repo: repo}
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/tasks" {
		switch r.Method {
		case http.MethodPost:
			h.handleCreate(w, r)
		case http.MethodGet:
			h.handleList(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}

		return
	}

	if !strings.HasPrefix(r.URL.Path, "/tasks/") {
		http.NotFound(w, r)
		return
	}

	Id := strings.TrimPrefix(r.URL.Path, "/tasks/")
	if Id == "" || strings.Contains(Id, "/") {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r, Id)
	case http.MethodPatch:
		h.handlePatch(w, r, Id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *httpHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Title *string `json:"title"`
	}

	if err := decodeStrictJSON(r.Body, &request); err != nil || request.Title == nil {
		http.Error(w, "invalId request body", http.StatusBadRequest)
		return
	}

	task, err := h.repo.Create(*request.Title)
	if err != nil {
		if errors.Is(err, ErrInvalIdTitle) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

func (h *httpHandler) handleGet(w http.ResponseWriter, r *http.Request, Id string) {
	task, ok := h.repo.Get(Id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func (h *httpHandler) handleList(w http.ResponseWriter, r *http.Request) {
	done := false
	if values, exists := r.URL.Query()["done"]; exists {
		if len(values) != 1 {
			http.Error(w, "invalId done query parameter", http.StatusBadRequest)
			return
		}

		switch values[0] {
		case "true":
			done = true
		case "false":
		default:
			http.Error(w, "invalId done query parameter", http.StatusBadRequest)
			return
		}
	}

	tasks := h.repo.List(done)
	if tasks == nil {
		tasks = make([]Task, 0)
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].UpdatedAt.Equal(tasks[j].UpdatedAt) {
			return tasks[i].Id < tasks[j].Id
		}
		return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
	})
	writeJSON(w, http.StatusOK, tasks)
}

func (h *httpHandler) handlePatch(w http.ResponseWriter, r *http.Request, Id string) {
	var request struct {
		Done *bool `json:"done"`
	}

	if err := decodeStrictJSON(r.Body, &request); err != nil || request.Done == nil {
		http.Error(w, "invalId request body", http.StatusBadRequest)
		return
	}

	task, err := h.repo.SetDone(Id, *request.Done)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.NotFound(w, r)
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func decodeStrictJSON(r io.Reader, v any) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}

		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
