// Package fakepanel is an in-memory fake of the Pterodactyl panel API for tests.
package fakepanel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

type Backup struct {
	UUID       string
	CreatedAt  time.Time
	Successful bool
	Locked     bool
	Completed  bool
}

// counted reports whether the panel counts the backup towards the server's limit
func (b Backup) counted() bool {
	return b.Successful || !b.Completed
}

type Panel struct {
	APIKey   string
	PageSize int

	// Servers holds the raw server objects returned by the application API
	Servers []json.RawMessage
	// Limits holds the backup limit per server identifier
	Limits map[string]int

	// FailNewBackups makes new backups complete unsuccessfully
	FailNewBackups bool
	// NeverComplete keeps new backups in progress forever
	NeverComplete bool
	// PollErrors makes the first PollErrors requests for a single backup fail with PollErrorStatus
	PollErrors      int
	PollErrorStatus int

	mu          sync.Mutex
	backups     map[string][]Backup
	events      []string
	nextID      int
	clock       time.Time
	polls       map[string]int
	pollsFailed int
}

// pollsToComplete is how many times a new backup is polled before it completes
const pollsToComplete = 3

// Start serves the panel until the test ends and returns its URL
func (p *Panel) Start(t testing.TB) string {
	if p.PageSize == 0 {
		p.PageSize = 2
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/application/servers", p.listServers)
	mux.HandleFunc("GET /api/client/servers/{server}/backups", p.listBackups)
	mux.HandleFunc("POST /api/client/servers/{server}/backups", p.createBackup)
	mux.HandleFunc("GET /api/client/servers/{server}/backups/{uuid}", p.getBackup)
	mux.HandleFunc("DELETE /api/client/servers/{server}/backups/{uuid}", p.deleteBackup)

	srv := httptest.NewServer(p.auth(mux))
	t.Cleanup(srv.Close)

	return srv.URL
}

// Server returns a raw application API server object
func Server(identifier string, backupLimit int) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"object":"server","attributes":{"identifier":%q,"name":%q,"feature_limits":{"backups":%d}}}`,
		identifier, identifier, backupLimit,
	))
}

// AddBackup stores an existing backup for server
func (p *Panel) AddBackup(server string, backup Backup) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.backups == nil {
		p.backups = map[string][]Backup{}
	}
	p.backups[server] = append(p.backups[server], backup)
}

// Backups returns the UUIDs of the backups stored for server, oldest first
func (p *Panel) Backups(server string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	var uuids []string
	for _, backup := range p.sorted(server) {
		uuids = append(uuids, backup.UUID)
	}

	return uuids
}

// Events returns the creates and deletes in the order they happened, like "create s1/new-1"
func (p *Panel) Events() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return slices.Clone(p.events)
}

func (p *Panel) sorted(server string) []Backup {
	backups := slices.Clone(p.backups[server])
	slices.SortFunc(backups, func(a, b Backup) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	return backups
}

func (p *Panel) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.APIKey != "" && r.Header.Get("Authorization") != "Bearer "+p.APIKey {
			writeError(w, http.StatusUnauthorized, "Unauthenticated.")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p *Panel) listServers(w http.ResponseWriter, r *http.Request) {
	writeList(w, r, p.PageSize, p.Servers)
}

func (p *Panel) listBackups(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var objects []json.RawMessage
	// like the real panel, the list is not sorted
	for _, backup := range p.backups[r.PathValue("server")] {
		objects = append(objects, backupObject(backup))
	}

	writeList(w, r, p.PageSize, objects)
}

func (p *Panel) createBackup(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()

	server := r.PathValue("server")

	var counted int
	for _, backup := range p.backups[server] {
		if backup.counted() {
			counted++
		}
	}

	if counted >= p.Limits[server] {
		writeError(w, http.StatusBadRequest, "Cannot create a new backup, this server has reached its limit.")
		return
	}

	if p.backups == nil {
		p.backups = map[string][]Backup{}
	}
	if p.clock.IsZero() {
		p.clock = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	p.nextID++
	p.clock = p.clock.Add(time.Hour)
	backup := Backup{UUID: "new-" + strconv.Itoa(p.nextID), CreatedAt: p.clock}
	p.backups[server] = append(p.backups[server], backup)
	p.events = append(p.events, "create "+server+"/"+backup.UUID)

	w.Write(backupObject(backup))
}

func (p *Panel) getBackup(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pollsFailed < p.PollErrors {
		p.pollsFailed++
		writeError(w, p.PollErrorStatus, "Injected poll error.")
		return
	}

	server, uuid := r.PathValue("server"), r.PathValue("uuid")

	i := slices.IndexFunc(p.backups[server], func(b Backup) bool { return b.UUID == uuid })
	if i < 0 {
		writeError(w, http.StatusNotFound, "The requested resource could not be found.")
		return
	}

	if p.polls == nil {
		p.polls = map[string]int{}
	}
	p.polls[uuid]++

	backup := &p.backups[server][i]
	if !backup.Completed && !p.NeverComplete && p.polls[uuid] >= pollsToComplete {
		backup.Completed = true
		backup.Successful = !p.FailNewBackups
	}

	w.Write(backupObject(*backup))
}

func (p *Panel) deleteBackup(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()

	server, uuid := r.PathValue("server"), r.PathValue("uuid")

	i := slices.IndexFunc(p.backups[server], func(b Backup) bool { return b.UUID == uuid })
	if i < 0 {
		writeError(w, http.StatusNotFound, "The requested resource could not be found.")
		return
	}

	if p.backups[server][i].Locked {
		writeError(w, http.StatusBadRequest, "Cannot delete a backup that is marked as locked.")
		return
	}

	p.backups[server] = slices.Delete(p.backups[server], i, i+1)
	p.events = append(p.events, "delete "+server+"/"+uuid)

	w.WriteHeader(http.StatusNoContent)
}

func backupObject(backup Backup) json.RawMessage {
	attributes := map[string]any{
		"uuid":          backup.UUID,
		"is_successful": backup.Successful,
		"is_locked":     backup.Locked,
		"name":          backup.UUID,
		"checksum":      nil,
		"bytes":         0,
		"created_at":    backup.CreatedAt,
		"completed_at":  nil,
	}
	if backup.Completed {
		attributes["completed_at"] = backup.CreatedAt.Add(time.Minute)
	}
	if backup.Successful {
		attributes["checksum"] = "sha1:" + backup.UUID
	}

	object, _ := json.Marshal(map[string]any{"object": "backup", "attributes": attributes})

	return object
}

func writeList(w http.ResponseWriter, r *http.Request, pageSize int, objects []json.RawMessage) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)

	totalPages := (len(objects) + pageSize - 1) / pageSize
	start := min((page-1)*pageSize, len(objects))
	end := min(start+pageSize, len(objects))

	data := objects[start:end]
	if data == nil {
		data = []json.RawMessage{}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   data,
		"meta": map[string]any{
			"pagination": map[string]any{
				"total":        len(objects),
				"count":        len(data),
				"per_page":     pageSize,
				"current_page": page,
				"total_pages":  totalPages,
			},
		},
	})
}

func writeError(w http.ResponseWriter, status int, detail string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]any{{"status": strconv.Itoa(status), "detail": detail}},
	})
}
