package handlers

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /admin/\nDisallow: /api/\nSitemap: %s/sitemap.xml\n", baseURL(r))
}

func (h *Handler) Sitemap(w http.ResponseWriter, r *http.Request) {
	type url struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod,omitempty"`
		ChangeFreq string `xml:"changefreq"`
		Priority   string `xml:"priority"`
	}
	type urlset struct {
		XMLName xml.Name `xml:"urlset"`
		NS      string   `xml:"xmlns,attr"`
		URLs    []url    `xml:"url"`
	}

	base := baseURL(r)
	urls := []url{
		{Loc: base + "/", ChangeFreq: "weekly", Priority: "1.0"},
		{Loc: base + "/about", ChangeFreq: "monthly", Priority: "0.8"},
		{Loc: base + "/blog", ChangeFreq: "weekly", Priority: "0.9"},
		{Loc: base + "/biking", ChangeFreq: "daily", Priority: "0.7"},
		{Loc: base + "/hardware", ChangeFreq: "monthly", Priority: "0.6"},
	}
	for _, p := range h.Blog.All() {
		urls = append(urls, url{
			Loc:        base + "/blog/" + p.Slug,
			LastMod:    p.Date.Format("2006-01-02"),
			ChangeFreq: "monthly",
			Priority:   "0.7",
		})
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(urlset{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls})
}

func (h *Handler) RSS(w http.ResponseWriter, r *http.Request) {
	base := baseURL(r)
	posts := h.Blog.All()

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>Blake Ridgway</title>
    <link>%s</link>
    <description>SRE practices, cloud infrastructure, DevOps automation, and cycling.</description>
    <language>en-us</language>
    <atom:link href="%s/feed.xml" rel="self" type="application/rss+xml"/>
    <lastBuildDate>%s</lastBuildDate>
`, base, base, time.Now().UTC().Format(time.RFC1123Z))

	for _, p := range posts {
		fmt.Fprintf(w, `    <item>
      <title>%s</title>
      <link>%s/blog/%s</link>
      <guid>%s/blog/%s</guid>
      <description>%s</description>
      <pubDate>%s</pubDate>
      <category>%s</category>
      <author>%s</author>
    </item>
`, xmlEscape(p.Title), base, p.Slug, base, p.Slug,
			xmlEscape(p.Excerpt), p.Date.UTC().Format(time.RFC1123Z),
			xmlEscape(p.Category), xmlEscape(p.Author))
	}

	fmt.Fprintf(w, "  </channel>\n</rss>\n")
}

func baseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") == "" {
		scheme = "http"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

func xmlEscape(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			out = append(out, []byte("&amp;")...)
		case '<':
			out = append(out, []byte("&lt;")...)
		case '>':
			out = append(out, []byte("&gt;")...)
		case '"':
			out = append(out, []byte("&quot;")...)
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
