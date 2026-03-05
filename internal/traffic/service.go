package traffic

import (
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type PageView struct {
	ID           int
	IPAddress    string
	UserAgent    string
	Path         string
	Method       string
	Referrer     string
	Timestamp    time.Time
	ResponseTime float64
	StatusCode   int
	SessionID    string
}

type Stats struct {
	TotalPageViews int
	UniqueVisitors int
	AvgResponseMs  float64
	BounceRate     float64
}

type PageSummary struct {
	Path  string
	Views int
}

type RefSummary struct {
	Referrer string
	Count    int
}

type DailyTraffic struct {
	Date      time.Time
	PageViews int
	Sessions  int
}

type Service struct {
	db *sql.DB
}

func New(dbPath string) (*Service, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS page_views (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip_address TEXT NOT NULL DEFAULT '',
			user_agent TEXT,
			path TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL DEFAULT 'GET',
			referrer TEXT,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			response_time REAL NOT NULL DEFAULT 0,
			status_code INTEGER NOT NULL DEFAULT 200,
			session_id TEXT
		);
		CREATE TABLE IF NOT EXISTS unique_visitors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip_address TEXT NOT NULL,
			user_agent_hash TEXT NOT NULL,
			first_visit DATETIME NOT NULL,
			last_visit DATETIME NOT NULL,
			visit_count INTEGER NOT NULL DEFAULT 1,
			UNIQUE(ip_address, user_agent_hash)
		);
		CREATE INDEX IF NOT EXISTS idx_pv_timestamp ON page_views(timestamp);
		CREATE INDEX IF NOT EXISTS idx_pv_path ON page_views(path);
	`)
	if err != nil {
		return nil, fmt.Errorf("traffic: create tables: %w", err)
	}

	return &Service{db: db}, nil
}

func (s *Service) Track(pv PageView) {
	_, _ = s.db.Exec(
		`INSERT INTO page_views (ip_address, user_agent, path, method, referrer, timestamp, response_time, status_code, session_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pv.IPAddress, pv.UserAgent, pv.Path, pv.Method, pv.Referrer,
		pv.Timestamp.UTC().Format("2006-01-02 15:04:05"),
		pv.ResponseTime, pv.StatusCode, pv.SessionID,
	)

	hash := sha256Hash(pv.UserAgent)
	_, _ = s.db.Exec(`
		INSERT INTO unique_visitors (ip_address, user_agent_hash, first_visit, last_visit, visit_count)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(ip_address, user_agent_hash) DO UPDATE SET
			last_visit = excluded.last_visit,
			visit_count = visit_count + 1`,
		pv.IPAddress, hash,
		pv.Timestamp.UTC().Format("2006-01-02 15:04:05"),
		pv.Timestamp.UTC().Format("2006-01-02 15:04:05"),
	)
}

func (s *Service) GetStats(days int) Stats {
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	var stats Stats
	_ = s.db.QueryRow(`SELECT COUNT(*), COALESCE(AVG(response_time),0) FROM page_views WHERE timestamp >= ?`, since).
		Scan(&stats.TotalPageViews, &stats.AvgResponseMs)

	_ = s.db.QueryRow(`SELECT COUNT(*) FROM unique_visitors WHERE last_visit >= ?`, since).
		Scan(&stats.UniqueVisitors)

	var total, bounces int
	rows, err := s.db.Query(`SELECT session_id, COUNT(*) FROM page_views WHERE timestamp >= ? GROUP BY session_id`, since)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sid string
			var cnt int
			if rows.Scan(&sid, &cnt) == nil {
				total++
				if cnt == 1 {
					bounces++
				}
			}
		}
	}
	if total > 0 {
		stats.BounceRate = float64(bounces) / float64(total) * 100
	}
	return stats
}

func (s *Service) TopPages(days, count int) []PageSummary {
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	rows, err := s.db.Query(
		`SELECT path, COUNT(*) as views FROM page_views WHERE timestamp >= ? GROUP BY path ORDER BY views DESC LIMIT ?`,
		since, count,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []PageSummary
	for rows.Next() {
		var p PageSummary
		if rows.Scan(&p.Path, &p.Views) == nil {
			result = append(result, p)
		}
	}
	return result
}

func (s *Service) TopReferrers(days, count int) []RefSummary {
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	rows, err := s.db.Query(
		`SELECT referrer, COUNT(*) as cnt FROM page_views WHERE timestamp >= ? AND referrer != '' GROUP BY referrer ORDER BY cnt DESC LIMIT ?`,
		since, count,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []RefSummary
	for rows.Next() {
		var r RefSummary
		if rows.Scan(&r.Referrer, &r.Count) == nil {
			result = append(result, r)
		}
	}
	return result
}

func (s *Service) DailyTraffic(days int) []DailyTraffic {
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := s.db.Query(
		`SELECT date(timestamp) as d, COUNT(*), COUNT(DISTINCT session_id) FROM page_views WHERE date(timestamp) >= ? GROUP BY d ORDER BY d`,
		since,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []DailyTraffic
	for rows.Next() {
		var dt DailyTraffic
		var d string
		if rows.Scan(&d, &dt.PageViews, &dt.Sessions) == nil {
			dt.Date, _ = time.Parse("2006-01-02", d)
			result = append(result, dt)
		}
	}
	return result
}

func (s *Service) RecentViews(count int) []PageView {
	rows, err := s.db.Query(
		`SELECT id, ip_address, COALESCE(user_agent,''), path, method, COALESCE(referrer,''), timestamp, response_time, status_code, COALESCE(session_id,'')
		 FROM page_views ORDER BY timestamp DESC LIMIT ?`, count,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []PageView
	for rows.Next() {
		var pv PageView
		var ts string
		if rows.Scan(&pv.ID, &pv.IPAddress, &pv.UserAgent, &pv.Path, &pv.Method,
			&pv.Referrer, &ts, &pv.ResponseTime, &pv.StatusCode, &pv.SessionID) == nil {
			pv.Timestamp, _ = time.Parse("2006-01-02 15:04:05", ts)
			result = append(result, pv)
		}
	}
	return result
}

// Middleware helpers

var excludedPaths = []string{"/health", "/_framework", "/_blazor", "/css", "/js", "/images", "/favicon", "/static"}
var excludedExt = map[string]bool{
	".css": true, ".js": true, ".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".svg": true, ".ico": true, ".woff": true, ".woff2": true,
	".ttf": true, ".map": true,
}

func ShouldTrack(path string) bool {
	for _, ex := range excludedPaths {
		if strings.HasPrefix(path, ex) {
			return false
		}
	}
	ext := filepath.Ext(path)
	return ext == "" || !excludedExt[ext]
}

func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return rip
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i]
	}
	return addr
}

func SessionID(r *http.Request) string {
	if c, err := r.Cookie("site_session"); err == nil && c.Value != "" {
		return c.Value
	}
	h := md5.Sum([]byte(fmt.Sprintf("%s-%s-%d", ClientIP(r), r.UserAgent(), time.Now().UnixNano())))
	return hex.EncodeToString(h[:])
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}
