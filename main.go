package main

import (
	"log"
	"net/http"
	"time"

	"ridgway.dev/personal/config"
	"ridgway.dev/personal/handlers"
	"ridgway.dev/personal/internal/auth"
	"ridgway.dev/personal/internal/blog"
	"ridgway.dev/personal/internal/cycling"
	"ridgway.dev/personal/internal/traffic"
	"ridgway.dev/personal/middleware"
)

func main() {
	cfg := config.Load()

	// Services
	blogSvc := blog.New(cfg.PostsDir, time.Duration(cfg.CacheHours)*time.Hour)
	cyclingSvc := cycling.New(cfg.IntervalsKey, cfg.IntervalsID, cfg.IntervalsURL, time.Duration(cfg.CacheHours)*time.Hour)
	trafficSvc, err := traffic.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("traffic db: %v", err)
	}
	authSvc := auth.New(cfg.AdminUser, cfg.AdminHash, cfg.SessionSecret)

	h := handlers.New(blogSvc, cyclingSvc, trafficSvc, authSvc, cfg.TemplatesDir, cfg.Dev)

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(cfg.StaticDir))))

	// Public pages
	mux.HandleFunc("GET /", h.Home)
	mux.HandleFunc("GET /about", h.About)
	mux.HandleFunc("GET /hardware", h.Hardware)
	mux.HandleFunc("GET /biking", h.Biking)
	mux.HandleFunc("GET /blog", h.BlogList)
	mux.HandleFunc("GET /blog/{slug}", h.BlogPost)

	// Feeds & system
	mux.HandleFunc("GET /feed.xml", h.RSS)
	mux.HandleFunc("GET /sitemap.xml", h.Sitemap)
	mux.HandleFunc("GET /robots.txt", h.RobotsTxt)
	mux.HandleFunc("GET /health", h.Health)

	// Admin — public
	mux.HandleFunc("GET /admin/login", h.AdminLogin)
	mux.HandleFunc("POST /admin/login", h.AdminLoginPost)
	mux.HandleFunc("GET /admin/logout", h.AdminLogout)

	// Admin — protected
	adminOnly := middleware.AdminOnly(authSvc)
	mux.Handle("GET /admin", adminOnly(http.HandlerFunc(h.AdminDashboard)))
	mux.Handle("GET /admin/traffic", adminOnly(http.HandlerFunc(h.AdminTraffic)))
	mux.Handle("GET /admin/blog/new", adminOnly(http.HandlerFunc(h.BlogNew)))
	mux.Handle("GET /admin/blog/{slug}/edit", adminOnly(http.HandlerFunc(h.BlogEdit)))
	mux.Handle("POST /admin/blog/save", adminOnly(http.HandlerFunc(h.BlogSave)))
	mux.Handle("POST /admin/blog/{slug}/delete", adminOnly(http.HandlerFunc(h.BlogDelete)))

	// Apply traffic tracking middleware to entire mux
	tracked := middleware.TrafficTracking(trafficSvc)(mux)

	host := ""
	if !cfg.Dev {
		host = "127.0.0.1"
	}
	addr := host + ":" + cfg.Port
	log.Printf("starting server on %s (dev=%v)", addr, cfg.Dev)
	if err := http.ListenAndServe(addr, tracked); err != nil {
		log.Fatal(err)
	}
}
