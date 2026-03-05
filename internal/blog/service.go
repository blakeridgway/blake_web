package blog

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

type Post struct {
	ID          int
	Slug        string
	Title       string
	Content     string        // raw markdown
	HTML        template.HTML // rendered HTML
	Excerpt     string
	Category    string
	Tags        []string
	Date        time.Time
	Author      string
	Draft       bool
	FilePath    string
	ReadMinutes int
}

type frontMatter struct {
	Title    string   `yaml:"title"`
	Slug     string   `yaml:"slug"`
	Date     string   `yaml:"date"`
	Category string   `yaml:"category"`
	Tags     []string `yaml:"tags"`
	Excerpt  string   `yaml:"excerpt"`
	Draft    bool     `yaml:"draft"`
	Author   string   `yaml:"author"`
}

var frontMatterRE = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)

type Service struct {
	postsDir string
	mu       sync.RWMutex
	cache    []Post
	cachedAt time.Time
	ttl      time.Duration
	md       goldmark.Markdown
}

func New(postsDir string, cacheTTL time.Duration) *Service {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
	)
	if err := os.MkdirAll(postsDir, 0755); err != nil {
		panic(fmt.Sprintf("blog: cannot create posts dir: %v", err))
	}
	return &Service{postsDir: postsDir, ttl: cacheTTL, md: md}
}

func (s *Service) All() []Post {
	s.mu.RLock()
	if time.Since(s.cachedAt) < s.ttl && s.cache != nil {
		posts := s.cache
		s.mu.RUnlock()
		return posts
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = s.load()
	s.cachedAt = time.Now()
	return s.cache
}

func (s *Service) Recent(n int) []Post {
	all := s.All()
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

func (s *Service) BySlug(slug string) (*Post, bool) {
	for _, p := range s.All() {
		if strings.EqualFold(p.Slug, slug) {
			return &p, true
		}
	}
	return nil, false
}

func (s *Service) Categories() []string {
	seen := map[string]bool{}
	var cats []string
	for _, p := range s.All() {
		if !seen[p.Category] {
			seen[p.Category] = true
			cats = append(cats, p.Category)
		}
	}
	sort.Strings(cats)
	return cats
}

func (s *Service) Save(p *Post) error {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", p.Title))
	sb.WriteString(fmt.Sprintf("slug: %s\n", p.Slug))
	sb.WriteString(fmt.Sprintf("date: %s\n", p.Date.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("category: %s\n", p.Category))
	if len(p.Tags) > 0 {
		sb.WriteString("tags:\n")
		for _, t := range p.Tags {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
	}
	if p.Excerpt != "" {
		sb.WriteString(fmt.Sprintf("excerpt: %q\n", p.Excerpt))
	}
	sb.WriteString(fmt.Sprintf("author: %s\n", p.Author))
	if p.Draft {
		sb.WriteString("draft: true\n")
	}
	sb.WriteString("---\n\n")
	sb.WriteString(p.Content)

	fileName := fmt.Sprintf("%s-%s.md", p.Date.Format("2006-01-02"), p.Slug)
	newPath := filepath.Join(s.postsDir, fileName)

	if p.FilePath != "" && p.FilePath != newPath {
		os.Remove(p.FilePath)
	}

	if err := os.WriteFile(newPath, []byte(sb.String()), 0644); err != nil {
		return err
	}
	p.FilePath = newPath
	s.invalidate()
	return nil
}

func (s *Service) Delete(slug string) error {
	p, ok := s.BySlug(slug)
	if !ok {
		return nil
	}
	if err := os.Remove(p.FilePath); err != nil {
		return err
	}
	s.invalidate()
	return nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

func (s *Service) load() []Post {
	entries, err := os.ReadDir(s.postsDir)
	if err != nil {
		return nil
	}

	var posts []Post
	id := 1
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(s.postsDir, e.Name())
		p, err := s.parse(path, id)
		if err != nil || p.Draft {
			continue
		}
		posts = append(posts, p)
		id++
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
	return posts
}

func (s *Service) parse(path string, id int) (Post, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Post{}, err
	}

	content := string(raw)
	var fm frontMatter

	if m := frontMatterRE.FindStringSubmatch(content); m != nil {
		_ = yaml.Unmarshal([]byte(m[1]), &fm)
		content = content[len(m[0]):]
	}

	slug := fm.Slug
	date := time.Now()
	base := strings.TrimSuffix(filepath.Base(path), ".md")

	if slug == "" {
		if len(base) > 10 && base[4] == '-' && base[7] == '-' {
			if t, err := time.Parse("2006-01-02", base[:10]); err == nil {
				date = t
				slug = base[11:]
			} else {
				slug = base
			}
		} else {
			slug = base
		}
	}

	if fm.Date != "" {
		for _, layout := range []string{"2006-01-02", "January 2, 2006", "2006-01-02T15:04:05Z"} {
			if t, err := time.Parse(layout, fm.Date); err == nil {
				date = t
				break
			}
		}
	}

	var buf bytes.Buffer
	if err := s.md.Convert([]byte(strings.TrimSpace(content)), &buf); err != nil {
		return Post{}, err
	}

	title := fm.Title
	if title == "" {
		title = slugToTitle(slug)
	}
	author := fm.Author
	if author == "" {
		author = "Blake Ridgway"
	}
	category := fm.Category
	if category == "" {
		category = "Uncategorized"
	}
	excerpt := fm.Excerpt
	if excerpt == "" {
		excerpt = generateExcerpt(content, 150)
	}

	return Post{
		ID:          id,
		Slug:        slug,
		Title:       title,
		Content:     strings.TrimSpace(content),
		HTML:        template.HTML(buf.String()),
		Excerpt:     excerpt,
		Category:    category,
		Tags:        fm.Tags,
		Date:        date,
		Author:      author,
		Draft:       fm.Draft,
		FilePath:    path,
		ReadMinutes: readTime(content),
	}, nil
}

var stripMD = regexp.MustCompile(`[#*` + "`" + `\[\]()>-]`)
var multiSpace = regexp.MustCompile(`\s+`)

func generateExcerpt(md string, max int) string {
	text := stripMD.ReplaceAllString(md, "")
	text = strings.TrimSpace(multiSpace.ReplaceAllString(text, " "))
	if len(text) <= max {
		return text
	}
	return strings.TrimRight(text[:max], " ") + "..."
}

func readTime(content string) int {
	words := len(strings.Fields(content))
	mins := (words + 199) / 200
	if mins < 1 {
		return 1
	}
	return mins
}

func slugToTitle(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}
