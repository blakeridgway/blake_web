package handlers

import (
	"net/http"

	"ridgway.dev/personal/internal/blog"
	"ridgway.dev/personal/internal/cycling"
)

// --- Home ---

type homeData struct {
	BasePage
	Stats      cycling.Stats
	Activities []cycling.Activity
	Posts      []blog.Post
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.NotFound(w, r)
		return
	}
	h.render(w, "home", homeData{
		BasePage:   BasePage{Page: "home"},
		Stats:      h.Cycling.YTDStats(),
		Activities: h.Cycling.Recent(3),
		Posts:      h.Blog.Recent(3),
	})
}

// --- About ---

func (h *Handler) About(w http.ResponseWriter, r *http.Request) {
	h.render(w, "about", BasePage{Page: "about"})
}

// --- Hardware / Setup ---

func (h *Handler) Hardware(w http.ResponseWriter, r *http.Request) {
	h.render(w, "hardware", BasePage{Page: "setup"})
}

// --- Biking ---

type bikingData struct {
	BasePage
	Stats      cycling.Stats
	Activities []cycling.Activity
	IsLive     bool
	GoalPct    float64
	ElevK      float64
}

func (h *Handler) Biking(w http.ResponseWriter, r *http.Request) {
	stats := h.Cycling.YTDStats()
	goalPct := stats.Distance / 5000 * 100
	if goalPct > 100 {
		goalPct = 100
	}
	h.render(w, "biking", bikingData{
		BasePage:   BasePage{Page: "cycling"},
		Stats:      stats,
		Activities: h.Cycling.Recent(10),
		IsLive:     h.Cycling.Available(),
		GoalPct:    goalPct,
		ElevK:      float64(stats.Elevation) / 1000,
	})
}

// --- Error / Not Found ---

type errorData struct {
	BasePage
	Code    int
	Message string
}

func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	h.render(w, "error", errorData{Code: 404, Message: "page not found"})
}

func (h *Handler) InternalError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	h.render(w, "error", errorData{Code: 500, Message: "internal server error"})
}
