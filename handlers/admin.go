package handlers

import (
	"net/http"
	"strings"
	"time"

	"ridgway.dev/personal/internal/blog"
	"ridgway.dev/personal/internal/traffic"
)

// --- Login ---

type adminLoginData struct {
	Error string
}

func (h *Handler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	if h.Auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	h.renderAdmin(w, "admin_login", adminLoginData{
		Error: r.URL.Query().Get("error"),
	})
}

func (h *Handler) AdminLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/login?error=invalid", http.StatusFound)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if h.Auth.Authenticate(username, password) {
		h.Auth.SetCookie(w, username)
		http.Redirect(w, r, "/admin", http.StatusFound)
	} else {
		http.Redirect(w, r, "/admin/login?error=invalid", http.StatusFound)
	}
}

func (h *Handler) AdminLogout(w http.ResponseWriter, r *http.Request) {
	h.Auth.ClearCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// --- Dashboard ---

type adminDashData struct {
	Section string
	Posts   []blog.Post
	Stats   traffic.Stats
}

func (h *Handler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	h.renderAdmin(w, "admin_dashboard", adminDashData{
		Section: "dashboard",
		Posts:   h.Blog.All(),
		Stats:   h.Traffic.GetStats(30),
	})
}

// --- Blog Editor ---

type adminEditorData struct {
	Post    *blog.Post
	IsNew   bool
	Error   string
	Success string
}

func (h *Handler) BlogNew(w http.ResponseWriter, r *http.Request) {
	h.renderAdmin(w, "admin_editor", adminEditorData{IsNew: true})
}

func (h *Handler) BlogEdit(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	post, ok := h.Blog.BySlug(slug)
	if !ok {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	h.renderAdmin(w, "admin_editor", adminEditorData{Post: post})
}

func (h *Handler) BlogSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	dateStr := r.FormValue("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		date = time.Now()
	}

	tagsRaw := r.FormValue("tags")
	var tags []string
	for _, t := range strings.Split(tagsRaw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}

	filePath := r.FormValue("file_path")
	post := &blog.Post{
		Slug:     r.FormValue("slug"),
		Title:    r.FormValue("title"),
		Content:  r.FormValue("content"),
		Excerpt:  r.FormValue("excerpt"),
		Category: r.FormValue("category"),
		Tags:     tags,
		Date:     date,
		Author:   r.FormValue("author"),
		Draft:    r.FormValue("draft") == "on",
		FilePath: filePath,
	}
	if post.Author == "" {
		post.Author = "Blake Ridgway"
	}

	if err := h.Blog.Save(post); err != nil {
		h.renderAdmin(w, "admin_editor", adminEditorData{
			Post:  post,
			Error: "failed to save: " + err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/admin?saved=1", http.StatusFound)
}

func (h *Handler) BlogDelete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	_ = h.Blog.Delete(slug)
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// --- Traffic ---

type adminTrafficData struct {
	Section      string
	Stats        traffic.Stats
	TopPages     []traffic.PageSummary
	TopReferrers []traffic.RefSummary
	DailyTraffic []traffic.DailyTraffic
	Days         int
}

func (h *Handler) AdminTraffic(w http.ResponseWriter, r *http.Request) {
	h.renderAdmin(w, "admin_traffic", adminTrafficData{
		Section:      "traffic",
		Stats:        h.Traffic.GetStats(30),
		TopPages:     h.Traffic.TopPages(30, 10),
		TopReferrers: h.Traffic.TopReferrers(30, 10),
		DailyTraffic: h.Traffic.DailyTraffic(30),
		Days:         30,
	})
}
