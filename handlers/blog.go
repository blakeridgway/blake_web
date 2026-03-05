package handlers

import (
	"net/http"
	"strings"

	"ridgway.dev/personal/internal/blog"
)

type blogListData struct {
	BasePage
	Posts      []blog.Post
	Categories []string
	Category   string
	Query      string
}

func (h *Handler) BlogList(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	posts := h.Blog.All()
	if category != "" {
		var filtered []blog.Post
		for _, p := range posts {
			if strings.EqualFold(p.Category, category) {
				filtered = append(filtered, p)
			}
		}
		posts = filtered
	}
	if query != "" {
		q := strings.ToLower(query)
		var filtered []blog.Post
		for _, p := range posts {
			if strings.Contains(strings.ToLower(p.Title), q) ||
				strings.Contains(strings.ToLower(p.Excerpt), q) ||
				strings.Contains(strings.ToLower(p.Category), q) {
				filtered = append(filtered, p)
			}
		}
		posts = filtered
	}
	h.render(w, "blog", blogListData{
		BasePage:   BasePage{Page: "blog"},
		Posts:      posts,
		Categories: h.Blog.Categories(),
		Category:   category,
		Query:      query,
	})
}

type blogPostData struct {
	BasePage
	Post *blog.Post
}

func (h *Handler) BlogPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	post, ok := h.Blog.BySlug(slug)
	if !ok {
		h.NotFound(w, r)
		return
	}
	h.render(w, "blog_post", blogPostData{
		BasePage: BasePage{Page: "blog"},
		Post:     post,
	})
}
