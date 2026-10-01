package models

var ArticleCategories = [...]string{
	"Devlog",
	"Post-Mortem",
	"Shipped",
	"Game Room",
	"News",
}

type Article struct {
	ID         int64
	Title      string
	Slug       string
	Category   string
	Summary    string
	BodyMD     string
	CoverImage string
	Published  bool
	CreatedAt  string
}

type Highlight struct {
	ID        int64
	Title     string
	Kicker    string
	Summary   string
	Href      string
	Image     string
	SortOrder int
	Published bool
}

type Inquiry struct {
	ID        int64
	Name      string
	Email     string
	Subject   string
	Message   string
	CreatedAt string
	Read      bool
}

func CategorySlug(category string) string {
	result := make([]rune, 0, len(category))
	for _, char := range category {
		if char == ' ' {
			result = append(result, '-')
			continue
		}
		if char >= 'A' && char <= 'Z' {
			char += 'a' - 'A'
		}
		result = append(result, char)
	}
	return string(result)
}

func ResolveCategory(raw string) string {
	for _, category := range ArticleCategories {
		if CategorySlug(category) == raw {
			return category
		}
	}
	return ""
}
