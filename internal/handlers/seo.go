package handlers

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/labstack/echo/v4"
)

const xmlHeader = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"

var sitemapPages = []string{"/", "/dispatches", "/game-room", "/projects", "/about", "/contact"}

type sitemapDocument struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Location string `xml:"loc"`
	LastMod  string `xml:"lastmod,omitempty"`
}

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language"`
	LastBuild   string    `xml:"lastBuildDate"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        rssGUID `xml:"guid"`
	Description string  `xml:"description"`
	PubDate     string  `xml:"pubDate"`
}

type rssGUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

func NewSitemap(site views.SiteConfig, database *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		articles, err := db.ListAllPublishedArticles(c.Request().Context(), database)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Could not build sitemap")
		}

		document := sitemapDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
		for _, path := range sitemapPages {
			document.URLs = append(document.URLs, sitemapURL{Location: absoluteSiteURL(site.SiteURL, path)})
		}
		for _, article := range articles {
			lastmod := ""
			if parsed, ok := parseArticleTime(article.CreatedAt); ok {
				lastmod = parsed.Format(time.RFC3339)
			}
			document.URLs = append(document.URLs, sitemapURL{
				Location: absoluteSiteURL(site.SiteURL, "/dispatches/"+article.Slug),
				LastMod:  lastmod,
			})
		}
		return writeXML(c, "application/xml; charset=UTF-8", document)
	}
}

func Robots(site views.SiteConfig) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, "text/plain; charset=UTF-8")
		_, err := fmt.Fprintf(c.Response(), "User-agent: *\nDisallow: /admin\nSitemap: %s\n", absoluteSiteURL(site.SiteURL, "/sitemap.xml"))
		return err
	}
}

func NewFeed(site views.SiteConfig, database *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		articles, err := db.ListAllPublishedArticles(c.Request().Context(), database)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Could not build dispatch feed")
		}

		siteURL := strings.TrimRight(site.SiteURL, "/")
		channel := rssChannel{
			Title:       "Waitaminute Digital Dispatches",
			Link:        siteURL + "/dispatches",
			Description: "Published dispatches from Waitaminute Digital.",
			Language:    "en-us",
			LastBuild:   time.Now().UTC().Format(time.RFC1123Z),
			Items:       make([]rssItem, 0, len(articles)),
		}
		for _, article := range articles {
			link := absoluteSiteURL(site.SiteURL, "/dispatches/"+article.Slug)
			item := rssItem{
				Title:       article.Title,
				Link:        link,
				GUID:        rssGUID{IsPermaLink: true, Value: link},
				Description: article.Summary,
			}
			if published, ok := parseArticleTime(article.CreatedAt); ok {
				item.PubDate = published.Format(time.RFC1123Z)
			}
			channel.Items = append(channel.Items, item)
		}
		return writeXML(c, "application/rss+xml; charset=UTF-8", rssDocument{Version: "2.0", Channel: channel})
	}
}

func writeXML(c echo.Context, contentType string, document any) error {
	encoded, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode XML response: %w", err)
	}
	c.Response().Header().Set(echo.HeaderContentType, contentType)
	_, err = c.Response().Write([]byte(xmlHeader))
	if err != nil {
		return err
	}
	_, err = c.Response().Write(encoded)
	if err != nil {
		return err
	}
	_, err = c.Response().Write([]byte("\n"))
	return err
}

func absoluteSiteURL(siteURL string, path string) string {
	base, err := url.Parse(strings.TrimRight(siteURL, "/") + "/")
	if err != nil {
		return strings.TrimRight(siteURL, "/") + "/" + strings.TrimLeft(path, "/")
	}
	relative, err := url.Parse(strings.TrimLeft(path, "/"))
	if err != nil {
		return strings.TrimRight(siteURL, "/") + "/" + strings.TrimLeft(path, "/")
	}
	return base.ResolveReference(relative).String()
}

func parseArticleTime(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}
