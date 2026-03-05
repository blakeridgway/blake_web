package cycling

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Stats struct {
	Distance  float64
	Elevation int
	Hours     float64
	Count     int
	AvgSpeed  float64
}

type Activity struct {
	Name      string
	Distance  float64
	Elevation int
	Duration  string
	Date      string
	AvgSpeed  float64
}

type apiActivity struct {
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	Distance           float64 `json:"distance"`
	TotalElevationGain float64 `json:"total_elevation_gain"`
	MovingTime         float64 `json:"moving_time"`
	StartDate          string  `json:"start_date"`
	StartDateLocal     string  `json:"start_date_local"`
}

type Service struct {
	client    *http.Client
	apiKey    string
	athleteID string
	baseURL   string
	ttl       time.Duration

	mu          sync.RWMutex
	statsCache  *Stats
	actCache    []Activity
	statsCached time.Time
	actCached   time.Time
}

func New(apiKey, athleteID, baseURL string, ttl time.Duration) *Service {
	if baseURL == "" {
		baseURL = "https://intervals.icu/api/v1/"
	}
	return &Service{
		client:    &http.Client{Timeout: 12 * time.Second},
		apiKey:    apiKey,
		athleteID: athleteID,
		baseURL:   baseURL,
		ttl:       ttl,
	}
}

func (s *Service) available() bool {
	return s.apiKey != "" && s.athleteID != ""
}

func (s *Service) YTDStats() Stats {
	s.mu.RLock()
	if s.statsCache != nil && time.Since(s.statsCached) < s.ttl {
		v := *s.statsCache
		s.mu.RUnlock()
		return v
	}
	s.mu.RUnlock()

	if !s.available() {
		return fallbackStats()
	}

	activities, err := s.fetchActivities(time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(activities) == 0 {
		return fallbackStats()
	}

	stats := calcStats(activities)
	s.mu.Lock()
	s.statsCache = &stats
	s.statsCached = time.Now()
	s.mu.Unlock()
	return stats
}

func (s *Service) Recent(count int) []Activity {
	s.mu.RLock()
	if s.actCache != nil && time.Since(s.actCached) < s.ttl {
		acts := s.actCache
		s.mu.RUnlock()
		if count > len(acts) {
			count = len(acts)
		}
		return acts[:count]
	}
	s.mu.RUnlock()

	if !s.available() {
		return fallbackActivities()
	}

	raw, err := s.fetchActivities(time.Now().AddDate(0, -3, 0))
	if err != nil || len(raw) == 0 {
		return fallbackActivities()
	}

	acts := formatActivities(raw, count)
	if len(acts) == 0 {
		return fallbackActivities()
	}
	s.mu.Lock()
	s.actCache = acts
	s.actCached = time.Now()
	s.mu.Unlock()
	return acts
}

func (s *Service) fetchActivities(since time.Time) ([]apiActivity, error) {
	url := fmt.Sprintf("%sathlete/%s/activities?oldest=%s", s.baseURL, s.athleteID, since.Format("2006-01-02"))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	creds := base64.StdEncoding.EncodeToString([]byte("API_KEY:" + s.apiKey))
	req.Header.Set("Authorization", "Basic "+creds)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("intervals.icu: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var acts []apiActivity
	if err := json.Unmarshal(body, &acts); err != nil {
		return nil, err
	}
	return acts, nil
}

func isCycling(t string) bool {
	l := strings.ToLower(t)
	return strings.Contains(l, "ride") || strings.Contains(l, "cycl") || strings.Contains(l, "bike")
}

func metersToMiles(m float64) float64 { return m * 0.000621371 }
func metersToFeet(m float64) float64  { return m * 3.28084 }

func formatDuration(seconds float64) string {
	s := int(seconds)
	if s <= 0 {
		return "0m"
	}
	h := s / 3600
	m := (s % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func calcStats(acts []apiActivity) Stats {
	var dist, elev, secs float64
	var count int
	for _, a := range acts {
		if !isCycling(a.Type) {
			continue
		}
		dist += a.Distance
		elev += a.TotalElevationGain
		secs += a.MovingTime
		count++
	}
	miles := metersToMiles(dist)
	hrs := secs / 3600
	avg := 0.0
	if hrs > 0 {
		avg = round1(miles / hrs)
	}
	return Stats{
		Distance:  round1(miles),
		Elevation: int(metersToFeet(elev) + 0.5),
		Hours:     round1(hrs),
		Count:     count,
		AvgSpeed:  avg,
	}
}

func formatActivities(acts []apiActivity, count int) []Activity {
	var cycling []apiActivity
	for _, a := range acts {
		if isCycling(a.Type) {
			cycling = append(cycling, a)
		}
	}
	// sort by date desc
	for i := 0; i < len(cycling)-1; i++ {
		for j := i + 1; j < len(cycling); j++ {
			if cycling[j].StartDateLocal > cycling[i].StartDateLocal {
				cycling[i], cycling[j] = cycling[j], cycling[i]
			}
		}
	}
	if count > len(cycling) {
		count = len(cycling)
	}
	cycling = cycling[:count]

	result := make([]Activity, 0, len(cycling))
	for _, a := range cycling {
		miles := metersToMiles(a.Distance)
		hrs := a.MovingTime / 3600
		avg := 0.0
		if hrs > 0 {
			avg = round1(miles / hrs)
		}
		dateStr := a.StartDateLocal
		if dateStr == "" {
			dateStr = a.StartDate
		}
		date := time.Now()
		if t, err := time.Parse("2006-01-02T15:04:05", dateStr); err == nil {
			date = t
		}
		result = append(result, Activity{
			Name:      a.Name,
			Distance:  round1(miles),
			Elevation: int(metersToFeet(a.TotalElevationGain) + 0.5),
			Duration:  formatDuration(a.MovingTime),
			Date:      date.Format("January 2, 2006"),
			AvgSpeed:  avg,
		})
	}
	return result
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// Available returns true when live API credentials are configured.
func (s *Service) Available() bool { return s.available() }

func fallbackStats() Stats {
	return Stats{Distance: 2450.5, Elevation: 45600, Hours: 156.2, Count: 127, AvgSpeed: 15.7}
}

func fallbackActivities() []Activity {
	return []Activity{
		{Name: "Morning Endurance Ride", Distance: 42.5, Elevation: 1250, Duration: "2h 38m", Date: time.Now().AddDate(0, 0, -1).Format("January 2, 2006"), AvgSpeed: 16.2},
		{Name: "Hill Intervals", Distance: 28.3, Elevation: 2100, Duration: "1h 55m", Date: time.Now().AddDate(0, 0, -3).Format("January 2, 2006"), AvgSpeed: 14.8},
		{Name: "Recovery Spin", Distance: 18.7, Elevation: 450, Duration: "1h 14m", Date: time.Now().AddDate(0, 0, -4).Format("January 2, 2006"), AvgSpeed: 15.1},
	}
}
