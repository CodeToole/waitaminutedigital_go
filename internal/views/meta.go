package views

import (
	"net/url"
	"strconv"
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

func DispatchesURL(category string, page int) string {
	query := url.Values{}
	if category != "" {
		query.Set("category", category)
	}
	if page > 1 {
		query.Set("page", strconv.Itoa(page))
	}
	if encoded := query.Encode(); encoded != "" {
		return "/dispatches?" + encoded
	}
	return "/dispatches"
}

func XShareURL(pageURL string, title string) string {
	return "https://twitter.com/intent/tweet?url=" + url.QueryEscape(pageURL) + "&text=" + url.QueryEscape(title)
}

func FacebookShareURL(pageURL string) string {
	return "https://www.facebook.com/sharer/sharer.php?u=" + url.QueryEscape(pageURL)
}

func LinkedInShareURL(pageURL string) string {
	return "https://www.linkedin.com/sharing/share-offsite/?url=" + url.QueryEscape(pageURL)
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
