package views

import (
	"strings"
	"time"
)

const (
	siteName           = "Waitaminute Digital"
	defaultDescription = "Indie game dev, Python, and problem-first software from Waitaminute Digital."
	defaultImage       = "/static/img/mascot_head.webp"
)

// PageMeta contains the page-specific SEO values consumed by Layout.
type PageMeta struct {
	Title         string
	Description   string
	Path          string
	CanonicalPath string
	Image         string
	OGType        string
	Canonical     string
	ImageURL      string
}

// NewPageMeta applies site defaults and resolves share URLs against SITE_URL.
func NewPageMeta(siteURL string, meta PageMeta) PageMeta {
	if meta.Title == "" {
		meta.Title = siteName
	} else if meta.Title != siteName && !strings.HasSuffix(meta.Title, " · "+siteName) {
		meta.Title += " · " + siteName
	}
	if meta.Description == "" {
		meta.Description = defaultDescription
	}
	if meta.Image == "" {
		meta.Image = defaultImage
	}
	if meta.OGType == "" {
		meta.OGType = "website"
	}
	canonicalPath := meta.CanonicalPath
	if canonicalPath == "" {
		canonicalPath = meta.Path
	}
	meta.Canonical = absoluteURL(siteURL, canonicalPath)
	meta.ImageURL = absoluteURL(siteURL, meta.Image)
	return meta
}

func IsCurrent(path string, href string) bool {
	return path == href || strings.HasPrefix(path, href+"/")
}

func CopyrightYear() string {
	return time.Now().UTC().Format("2006")
}

func CategoryURL(slug string) string {
	if slug == "" {
		return "/"
	}
	return "/?category=" + slug
}

func FormatDate(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.Format("02 Jan 2006")
}

func absoluteURL(siteURL string, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(siteURL, "/") + "/" + strings.TrimLeft(path, "/")
}
