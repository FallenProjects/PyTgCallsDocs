package docs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"regexp"
	"sort"
	"strings"
)

type Catalog struct {
	Ref     string   `json:"ref"`
	Entries []*Entry `json:"entries"`
	byID    map[string]*Entry
}

type Entry struct {
	ID          string     `json:"id"`
	Path        string     `json:"path"`
	Title       string     `json:"title"`
	Library     string     `json:"library"`
	Kind        string     `json:"kind"`
	Description string     `json:"description,omitempty"`
	Preview     string     `json:"preview,omitempty"`
	URL         string     `json:"url"`
	Content     *Content   `json:"content,omitempty"`
	Languages   []Language `json:"languages,omitempty"`
}

type Language struct {
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	Content Content `json:"content"`
}

type Content struct {
	Signature string    `json:"signature,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Example   *Example  `json:"example,omitempty"`
	Sections  []Section `json:"sections,omitempty"`
}

type Example struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

type Section struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}

type Item struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

func ID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:8])
}

func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return &catalog, nil
}

func (c *Catalog) Validate() error {
	if c.Ref == "" {
		return errors.New("catalog has no source ref")
	}
	if len(c.Entries) == 0 {
		return errors.New("catalog has no entries")
	}
	c.byID = make(map[string]*Entry, len(c.Entries))
	paths := make(map[string]bool, len(c.Entries))
	for _, entry := range c.Entries {
		if entry == nil || entry.Path == "" || entry.Title == "" || !strings.HasPrefix(entry.URL, "https://") {
			return errors.New("catalog contains an incomplete entry")
		}
		if entry.Library != "PyTgCalls" && entry.Library != "NTgCalls" {
			return fmt.Errorf("unknown library for %s", entry.Path)
		}
		if entry.Preview != "" && entry.Description == "" {
			return fmt.Errorf("preview without description for %s", entry.Path)
		}
		if len(entry.Languages) > 0 && entry.Content != nil {
			return fmt.Errorf("duplicate default content for %s", entry.Path)
		}
		languageIDs := make(map[string]bool)
		for _, language := range entry.Languages {
			if language.ID == "" || languageIDs[language.ID] {
				return fmt.Errorf("missing or duplicate language for %s", entry.Path)
			}
			languageIDs[language.ID] = true
		}
		if entry.ID != ID(entry.Path) {
			return fmt.Errorf("invalid id for %s", entry.Path)
		}
		if paths[entry.Path] || c.byID[entry.ID] != nil {
			return fmt.Errorf("duplicate entry %s", entry.Path)
		}
		paths[entry.Path] = true
		c.byID[entry.ID] = entry
	}
	return nil
}

func (c *Catalog) ByID(id string) *Entry {
	return c.byID[id]
}

func (e *Entry) Selected(language string) (Content, string) {
	if len(e.Languages) == 0 {
		if e.Content != nil {
			return *e.Content, ""
		}
		return Content{}, ""
	}
	for _, variant := range e.Languages {
		if variant.ID == language {
			return variant.Content, variant.ID
		}
	}
	return e.Languages[0].Content, e.Languages[0].ID
}

var tags = regexp.MustCompile(`<[^>]*>`)
var blockSeparators = strings.NewReplacer("<br>", " ", "<hr/>", " ", "</li>", " ", "</ul>", " ", "</ol>", " ", "</h2>", " ", "</h3>", " ", "</pre>", " ", "</blockquote>", " ", "</th>", " ", "</td>", " ")

func Plain(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tags.ReplaceAllString(blockSeparators.Replace(s), ""))), " ")
}

func (c *Catalog) Search(query string, limit int) []*Entry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || limit <= 0 {
		return nil
	}
	type match struct {
		entry *Entry
		score int
	}
	var matches []match
	for _, entry := range c.Entries {
		title := strings.ToLower(entry.Title)
		score := 0
		switch {
		case title == query:
			score = 100
		case strings.HasPrefix(title, query):
			score = 70
		case strings.Contains(title, query):
			score = 50
		}
		if strings.Contains(strings.ToLower(entry.Path), query) {
			score += 20
		}
		if strings.Contains(strings.ToLower(entry.Preview), query) {
			score += 10
		}
		content := []Content{}
		if entry.Content != nil {
			content = append(content, *entry.Content)
		}
		for _, language := range entry.Languages {
			content = append(content, language.Content)
		}
		foundSignature, foundItem := false, false
		for _, part := range content {
			foundSignature = foundSignature || strings.Contains(strings.ToLower(Plain(part.Signature)), query)
			foundItem = foundItem || strings.Contains(strings.ToLower(Plain(part.Notes)), query)
			for _, section := range part.Sections {
				for _, item := range section.Items {
					foundItem = foundItem || strings.Contains(strings.ToLower(Plain(item.Name)+" "+Plain(item.Description)), query)
				}
			}
		}
		if foundSignature {
			score += 10
		}
		if foundItem {
			score += 5
		}
		if score > 0 {
			matches = append(matches, match{entry, score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].entry.Path < matches[j].entry.Path
		}
		return matches[i].score > matches[j].score
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	results := make([]*Entry, len(matches))
	for i, match := range matches {
		results[i] = match.entry
	}
	return results
}
