package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPort      = "8080"
	defaultCacheTTL  = 30 * time.Minute
	defaultGitHubUser = "bhargavms"
	defaultSOUser    = "4128945"
)

type cacheEntry struct {
	body      []byte
	expiresAt time.Time
}

type cache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
	ttl   time.Duration
}

func newCache(ttl time.Duration) *cache {
	return &cache{items: make(map[string]cacheEntry), ttl: ttl}
}

func (c *cache) get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.body, true
}

func (c *cache) set(key string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cacheEntry{body: body, expiresAt: time.Now().Add(c.ttl)}
}

type server struct {
	client      *http.Client
	cache       *cache
	githubToken string
	githubUser  string
	soUserID    string
	allowedOrigins map[string]struct{}
}

func main() {
	ttl := defaultCacheTTL
	if v := os.Getenv("CACHE_TTL_SECONDS"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			ttl = time.Duration(secs) * time.Second
		}
	}

	githubUser := envOr("GITHUB_USER", defaultGitHubUser)
	soUser := envOr("STACKOVERFLOW_USER_ID", defaultSOUser)

	origins := map[string]struct{}{
		"https://mogra.dev":      {},
		"http://localhost:1313":  {},
		"http://127.0.0.1:1313":  {},
	}
	if extra := os.Getenv("ALLOWED_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			origins[strings.TrimSpace(o)] = struct{}{}
		}
	}

	s := &server{
		client:      &http.Client{Timeout: 15 * time.Second},
		cache:       newCache(ttl),
		githubToken: os.Getenv("GITHUB_TOKEN"),
		githubUser:  githubUser,
		soUserID:    soUser,
		allowedOrigins: origins,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/github/repos", s.handleGitHubRepos)
	mux.HandleFunc("GET /api/github/repo/{owner}/{name}", s.handleGitHubRepo)
	mux.HandleFunc("GET /api/stackoverflow/answers", s.handleSOAnswers)
	mux.HandleFunc("GET /api/stackoverflow/questions", s.handleSOQuestions)

	port := envOr("PORT", defaultPort)
	log.Printf("mogra-proxy listening on :%s (cache TTL: %s)", port, ttl)
	if err := http.ListenAndServe(":"+port, withCORS(origins, mux)); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *server) handleGitHubRepos(w http.ResponseWriter, r *http.Request) {
	url := "https://api.github.com/users/" + s.githubUser + "/repos?sort=updated&per_page=30"
	s.proxyJSON(w, r, "github:repos", url, s.githubHeaders())
}

func (s *server) handleGitHubRepo(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	if owner == "" || name == "" {
		http.Error(w, "owner and name required", http.StatusBadRequest)
		return
	}
	url := "https://api.github.com/repos/" + owner + "/" + name
	s.proxyJSON(w, r, "github:repo:"+owner+"/"+name, url, s.githubHeaders())
}

func (s *server) handleSOAnswers(w http.ResponseWriter, r *http.Request) {
	pagesize := r.URL.Query().Get("pagesize")
	if pagesize == "" {
		pagesize = "10"
	}
	cacheKey := "so:answers:" + pagesize
	if body, ok := s.cache.get(cacheKey); ok {
		writeJSON(w, body)
		return
	}

	url := "https://api.stackexchange.com/2.3/users/" + s.soUserID +
		"/answers?order=desc&sort=votes&site=stackoverflow&pagesize=" + pagesize + "&filter=withbody"
	body, status, err := s.fetchURL(r.Context(), url, nil)
	if err != nil {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	if status >= 400 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
		return
	}

	enriched, err := s.enrichSOAnswers(body)
	if err != nil {
		http.Error(w, "failed to enrich answers", http.StatusBadGateway)
		return
	}

	s.cache.set(cacheKey, enriched)
	writeJSON(w, enriched)
}

func (s *server) handleSOQuestions(w http.ResponseWriter, r *http.Request) {
	pagesize := r.URL.Query().Get("pagesize")
	if pagesize == "" {
		pagesize = "10"
	}
	url := "https://api.stackexchange.com/2.3/users/" + s.soUserID +
		"/questions?order=desc&sort=votes&site=stackoverflow&pagesize=" + pagesize
	s.proxyJSON(w, r, "so:questions:"+pagesize, url, nil)
}

func (s *server) githubHeaders() http.Header {
	h := make(http.Header)
	h.Set("Accept", "application/vnd.github+json")
	h.Set("User-Agent", "mogra-proxy")
	if s.githubToken != "" {
		h.Set("Authorization", "Bearer "+s.githubToken)
	}
	return h
}

func (s *server) proxyJSON(w http.ResponseWriter, r *http.Request, cacheKey, url string, headers http.Header) {
	if body, ok := s.cache.get(cacheKey); ok {
		writeJSON(w, body)
		return
	}

	body, status, err := s.fetchURL(r.Context(), url, headers)
	if err != nil {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	if status >= 400 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
		return
	}
	if !json.Valid(body) {
		http.Error(w, "invalid upstream JSON", http.StatusBadGateway)
		return
	}

	s.cache.set(cacheKey, body)
	writeJSON(w, body)
}

func (s *server) fetchURL(ctx context.Context, url string, headers http.Header) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "mogra-proxy")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

func (s *server) enrichSOAnswers(body []byte) ([]byte, error) {
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if len(payload.Items) == 0 {
		return body, nil
	}

	questionIDs := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		switch qid := item["question_id"].(type) {
		case float64:
			questionIDs = append(questionIDs, strconv.Itoa(int(qid)))
		case int:
			questionIDs = append(questionIDs, strconv.Itoa(qid))
		}
	}
	if len(questionIDs) == 0 {
		return body, nil
	}

	questionsURL := "https://api.stackexchange.com/2.3/questions/" +
		strings.Join(questionIDs, ";") + "?site=stackoverflow&filter=default"
	qBody, status, err := s.fetchURL(context.Background(), questionsURL, nil)
	if err != nil || status >= 400 {
		return body, nil
	}

	var questions struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(qBody, &questions); err != nil {
		return body, nil
	}

	titles := make(map[int]string, len(questions.Items))
	for _, q := range questions.Items {
		id, _ := q["question_id"].(float64)
		title, _ := q["title"].(string)
		titles[int(id)] = title
	}

	for i, item := range payload.Items {
		qid, _ := item["question_id"].(float64)
		if title, ok := titles[int(qid)]; ok {
			payload.Items[i]["title"] = title
		}
		if aid, ok := item["answer_id"].(float64); ok {
			payload.Items[i]["link"] = fmt.Sprintf("https://stackoverflow.com/a/%d", int(aid))
		}
		if bodyHTML, ok := item["body"].(string); ok {
			payload.Items[i]["excerpt"] = stripHTML(bodyHTML)
		}
	}

	return json.Marshal(payload)
}

func stripHTML(input string) string {
	var b strings.Builder
	inTag := false
	for _, r := range input {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func writeJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func withCORS(allowed map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
