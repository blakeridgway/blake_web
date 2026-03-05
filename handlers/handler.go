package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"ridgway.dev/personal/internal/auth"
	"ridgway.dev/personal/internal/blog"
	"ridgway.dev/personal/internal/cycling"
	"ridgway.dev/personal/internal/traffic"
)

// Handler holds all service dependencies and the template registry.
type Handler struct {
	Blog    *blog.Service
	Cycling *cycling.Service
	Traffic *traffic.Service
	Auth    *auth.Service
	tmpl    map[string]*template.Template
	dev     bool
	tmplDir string
}

// BasePage is embedded in every page data struct for nav highlighting.
type BasePage struct {
	Page string // "home" | "blog" | "about" | "setup" | "cycling"
}

func New(
	blogSvc *blog.Service,
	cyclingSvc *cycling.Service,
	trafficSvc *traffic.Service,
	authSvc *auth.Service,
	templatesDir string,
	dev bool,
) *Handler {
	h := &Handler{
		Blog:    blogSvc,
		Cycling: cyclingSvc,
		Traffic: trafficSvc,
		Auth:    authSvc,
		dev:     dev,
		tmplDir: templatesDir,
	}
	h.tmpl = h.loadTemplates()
	return h
}

func (h *Handler) loadTemplates() map[string]*template.Template {
	funcs := template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"fmtDate":  func(t time.Time, layout string) string { return t.Format(layout) },
		"now":      time.Now,
		"add":      func(a, b int) int { return a + b },
		"sub":      func(a, b int) int { return a - b },
		"catClass": func(cat string) string {
			switch cat {
			case "tech", "technology", "infrastructure", "devops", "sre":
				return "tech"
			case "cycling", "biking", "endurance":
				return "cycling"
			default:
				return "general"
			}
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
	}

	base := filepath.Join(h.tmplDir, "base.html")
	adminBase := filepath.Join(h.tmplDir, "admin", "base.html")

	pages := map[string]string{
		"home":      "home.html",
		"blog":      "blog.html",
		"blog_post": "blog_post.html",
		"about":     "about.html",
		"hardware":  "hardware.html",
		"biking":    "biking.html",
		"error":     "error.html",
	}
	adminPages := map[string]string{
		"admin_login":     "admin/login.html",
		"admin_dashboard": "admin/dashboard.html",
		"admin_editor":    "admin/editor.html",
		"admin_traffic":   "admin/traffic.html",
	}

	tmpl := make(map[string]*template.Template)
	for name, page := range pages {
		path := filepath.Join(h.tmplDir, page)
		t, err := template.New("").Funcs(funcs).ParseFiles(base, path)
		if err != nil {
			log.Fatalf("template %q: %v", name, err)
		}
		tmpl[name] = t
	}
	for name, page := range adminPages {
		path := filepath.Join(h.tmplDir, page)
		t, err := template.New("").Funcs(funcs).ParseFiles(adminBase, path)
		if err != nil {
			log.Fatalf("template %q: %v", name, err)
		}
		tmpl[name] = t
	}
	return tmpl
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	if h.dev {
		// Reload templates on every request during development
		h.tmpl = h.loadTemplates()
	}
	t, ok := h.tmpl[name]
	if !ok {
		http.Error(w, fmt.Sprintf("template %q not found", name), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("render %q: %v", name, err)
	}
}

func (h *Handler) renderAdmin(w http.ResponseWriter, name string, data any) {
	if h.dev {
		h.tmpl = h.loadTemplates()
	}
	t, ok := h.tmpl[name]
	if !ok {
		http.Error(w, fmt.Sprintf("template %q not found", name), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "admin_base", data); err != nil {
		log.Printf("render %q: %v", name, err)
	}
}
