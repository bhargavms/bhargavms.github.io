package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
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
	defaultUmamiUpstream = "http://umami.umami.svc.cluster.local:3000"
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
	umamiProxy  *httputil.ReverseProxy
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

	umamiUpstream := envOr("UMAMI_UPSTREAM", defaultUmamiUpstream)
	umamiURL, err := url.Parse(umamiUpstream)
	if err != nil {
		log.Fatalf("invalid UMAMI_UPSTREAM: %v", err)
	}
	umamiProxy := httputil.NewSingleHostReverseProxy(umamiURL)

	s := &server{
		client:      &http.Client{Timeout: 15 * time.Second},
		cache:       newCache(ttl),
		githubToken: os.Getenv("GITHUB_TOKEN"),
		githubUser:  githubUser,
		soUserID:    soUser,
		allowedOrigins: origins,
		umamiProxy:  umamiProxy,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /api/send", s.handleUmamiSend)
	mux.HandleFunc("GET /api/github/repos", s.handleGitHubRepos)
	mux.HandleFunc("GET /api/github/repo/{owner}/{name}", s.handleGitHubRepo)
	mux.HandleFunc("GET /api/stackoverflow/answers", s.handleSOAnswers)
	mux.HandleFunc("GET /api/stackoverflow/questions", s.handleSOQuestions)
	mux.HandleFunc("GET /api/stackoverflow/summary", s.handleSOSummary)

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

func (s *server) handleUmamiSend(w http.ResponseWriter, r *http.Request) {
	s.umamiProxy.ServeHTTP(w, r)
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

var soPrivileges = []struct {
	Rep  int
	Name string
}{
	{15, "Vote up"},
	{50, "Comment everywhere"},
	{125, "Vote down"},
	{500, "Access review queues"},
	{1000, "See vote counts"},
	{2000, "Edit community wiki"},
	{3000, "Create tags"},
	{5000, "Access site analytics"},
	{10000, "Access moderator tools"},
	{15000, "Protect questions"},
	{20000, "Trusted user"},
	{25000, "Access to vote counts"},
}

func (s *server) handleSOSummary(w http.ResponseWriter, r *http.Request) {
	cacheKey := "so:summary:v3"
	if body, ok := s.cache.get(cacheKey); ok {
		writeJSON(w, body)
		return
	}

	ctx := r.Context()
	userParams := url.Values{}
	userParams.Set("site", "stackoverflow")
	userParams.Set("filter", "!-*f(6q9Y*ecs")
	userURL := "https://api.stackexchange.com/2.3/users/" + s.soUserID + "?" + userParams.Encode()

	userBody, status, err := s.fetchURL(ctx, userURL, nil)
	if err != nil || status >= 400 {
		http.Error(w, "failed to fetch user profile", http.StatusBadGateway)
		return
	}

	var userPayload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(userBody, &userPayload); err != nil || len(userPayload.Items) == 0 {
		http.Error(w, "invalid user profile", http.StatusBadGateway)
		return
	}
	user := userPayload.Items[0]

	base := "https://api.stackexchange.com/2.3/users/" + s.soUserID
	siteQuery := "site=stackoverflow"

	tagBody, _, _ := s.fetchURL(ctx, base+"/tags?"+siteQuery+"&pagesize=1&order=desc&sort=popular", nil)
	var tagPayload struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(tagBody, &tagPayload)

	reputation, _ := user["reputation"].(float64)
	repChangeWeek, _ := user["reputation_change_week"].(float64)
	profileLink, _ := user["link"].(string)

	var topTag map[string]any
	if len(tagPayload.Items) > 0 {
		item := tagPayload.Items[0]
		name, _ := item["name"].(string)
		count, _ := item["count"].(float64)
		topTag = map[string]any{"name": name, "count": int(count)}
	}

	nextPrivilege := nextSOPrivilege(int(reputation))
	peopleReached := scrapePeopleReached(ctx, s, profileLink)
	if peopleReached == "" {
		peopleReached = s.estimatePeopleReachedFromAPI(ctx)
	}

	summary := map[string]any{
		"reputation":             int(reputation),
		"reputation_change_week": int(repChangeWeek),
		"top_tag":                topTag,
		"next_privilege":         nextPrivilege,
		"people_reached":         peopleReached,
		"profile_link":           profileLink,
	}

	out, err := json.Marshal(summary)
	if err != nil {
		http.Error(w, "failed to encode summary", http.StatusInternalServerError)
		return
	}

	s.cache.set(cacheKey, out)
	writeJSON(w, out)
}

func nextSOPrivilege(reputation int) map[string]any {
	for _, p := range soPrivileges {
		if reputation < p.Rep {
			progress := float64(reputation) / float64(p.Rep)
			if progress > 1 {
				progress = 1
			}
			return map[string]any{
				"name":     p.Name,
				"rep":      p.Rep,
				"progress": progress,
			}
		}
	}
	last := soPrivileges[len(soPrivileges)-1]
	return map[string]any{
		"name":     last.Name,
		"rep":      last.Rep,
		"progress": 1.0,
	}
}

var (
	peopleReachedStatsPattern = regexp.MustCompile(`(?is)fs-body3[^>]*>\s*([~]?[\d,.]+[kmb]?)\s*</div>\s*reached`)
	peopleReachedPattern      = regexp.MustCompile(`(?is)>\s*([~]?[\d,.]+[kmb]?)\s*</div>\s*reached`)
	peopleReachedFallback     = regexp.MustCompile(`(?is)people viewed helpful posts.*?>\s*([~]?[\d,.]+[kmb]?)\s*</div>\s*reached`)
)

func scrapePeopleReached(ctx context.Context, s *server, profileURL string) string {
	if profileURL == "" {
		return ""
	}

	body, status, err := s.fetchURL(ctx, profileURL, http.Header{
		"User-Agent":      []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
		"Accept":          []string{"text/html,application/xhtml+xml"},
		"Accept-Language": []string{"en-US,en;q=0.9"},
	})
	if err != nil || status >= 400 {
		return ""
	}

	var match []byte
	switch {
	case peopleReachedStatsPattern.Match(body):
		match = peopleReachedStatsPattern.FindSubmatch(body)[1]
	case peopleReachedPattern.Match(body):
		match = peopleReachedPattern.FindSubmatch(body)[1]
	case peopleReachedFallback.Match(body):
		match = peopleReachedFallback.FindSubmatch(body)[1]
	default:
		return ""
	}

	return normalizePeopleReached(string(match))
}

func normalizePeopleReached(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "~") {
		value = "~" + value
	}
	return value
}

func (s *server) estimatePeopleReachedFromAPI(ctx context.Context) string {
	questionIDs := make(map[int]struct{})
	page := 1

	for {
		answersURL := fmt.Sprintf(
			"https://api.stackexchange.com/2.3/users/%s/answers?site=stackoverflow&pagesize=100&page=%d&order=desc&sort=votes&filter=default",
			s.soUserID, page,
		)
		body, status, err := s.fetchURL(ctx, answersURL, nil)
		if err != nil || status >= 400 {
			break
		}

		var payload struct {
			Items   []map[string]any `json:"items"`
			HasMore bool             `json:"has_more"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			break
		}

		for _, item := range payload.Items {
			score, _ := item["score"].(float64)
			if score <= 0 {
				continue
			}
			if qid, ok := item["question_id"].(float64); ok {
				questionIDs[int(qid)] = struct{}{}
			}
		}

		if !payload.HasMore {
			break
		}
		page++
	}

	if len(questionIDs) == 0 {
		return ""
	}

	ids := make([]string, 0, len(questionIDs))
	for id := range questionIDs {
		ids = append(ids, strconv.Itoa(id))
	}

	totalViews := 0
	for i := 0; i < len(ids); i += 100 {
		end := i + 100
		if end > len(ids) {
			end = len(ids)
		}
		questionsURL := "https://api.stackexchange.com/2.3/questions/" +
			strings.Join(ids[i:end], ";") + "?site=stackoverflow&filter=default"
		body, status, err := s.fetchURL(ctx, questionsURL, nil)
		if err != nil || status >= 400 {
			continue
		}

		var questions struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(body, &questions); err != nil {
			continue
		}
		for _, q := range questions.Items {
			if views, ok := q["view_count"].(float64); ok {
				totalViews += int(views)
			}
		}
	}

	if totalViews == 0 {
		return ""
	}
	return normalizePeopleReached(formatCompactViews(totalViews))
}

func formatCompactViews(views int) string {
	switch {
	case views >= 1_000_000:
		if views%1_000_000 == 0 {
			return fmt.Sprintf("%dm", views/1_000_000)
		}
		return fmt.Sprintf("%.1fm", float64(views)/1_000_000)
	case views >= 10_000:
		return fmt.Sprintf("%dk", views/1_000)
	case views >= 1_000:
		return fmt.Sprintf("%.1fk", float64(views)/1_000)
	default:
		return strconv.Itoa(views)
	}
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
