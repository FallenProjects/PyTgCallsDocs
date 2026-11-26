package bot

import (
	"ashokshau/pytgdocs/internal/docs"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"log/slog"
	"math/big"
	"regexp"
	"strings"

	"github.com/AshokShau/gotdbot"
)

var linkedTerm = regexp.MustCompile(`\+([^+\n]+)\+`)

func (b *Bot) linkedResults(client *gotdbot.Client, query, offset string, additional []gotdbot.InputInlineQueryResult) ([]gotdbot.InputInlineQueryResult, string) {
	if len(query) > 3000 {
		return nil, ""
	}
	matches := linkedTerm.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		return nil, ""
	}

	options, linkCount := b.linkChoices(query, matches)
	total := new(big.Int).Add(linkCount, big.NewInt(int64(len(additional))))

	index := new(big.Int)
	if _, ok := index.SetString(offset, 10); !ok || index.Sign() < 0 || index.Cmp(total) >= 0 {
		index.SetInt64(0)
	}
	results := make([]gotdbot.InputInlineQueryResult, 0, 40)
	for len(results) < 40 && index.Cmp(total) < 0 {
		if index.Cmp(linkCount) >= 0 {
			extraIndex := new(big.Int).Sub(index, linkCount)
			results = append(results, additional[int(extraIndex.Int64())])
		} else {
			selected := linkSelection(options, index)
			message, description := linkedMessage(query, matches, selected)
			formatted, err := client.GetFormattedText(message, nil, "HTML")
			if err != nil {
				slog.Warn("format linked message", "error", err)
			} else {
				title := "Link documentation pages"
				if noLinkedPages(selected) {
					title = "No matching documentation pages"
				}
				hash := sha256.Sum256([]byte(query + ":" + index.String()))
				results = append(results, &gotdbot.InputInlineQueryResultArticle{
					Id:          "linked_" + hex.EncodeToString(hash[:8]),
					Title:       title,
					Description: description,
					InputMessageContent: &gotdbot.InputMessageText{
						Text:               formatted,
						LinkPreviewOptions: &gotdbot.LinkPreviewOptions{IsDisabled: true},
					},
				})
			}
		}
		index.Add(index, big.NewInt(1))
	}
	if index.Cmp(total) < 0 {
		return results, index.String()
	}
	return results, ""
}

func (b *Bot) linkChoices(query string, matches [][]int) ([][]*docs.Entry, *big.Int) {
	options := make([][]*docs.Entry, len(matches))
	count := big.NewInt(1)
	for i, match := range matches {
		term := strings.TrimSpace(query[match[2]:match[3]])
		options[i] = b.linkOptions(term)
		if len(options[i]) == 0 {
			options[i] = []*docs.Entry{nil}
		}
		count.Mul(count, big.NewInt(int64(len(options[i]))))
	}
	return options, count
}

func noLinkedPages(selected []*docs.Entry) bool {
	for _, entry := range selected {
		if entry != nil {
			return false
		}
	}
	return true
}

func (b *Bot) linkOptions(term string) []*docs.Entry {
	found := b.Catalog.Search(term, len(b.Catalog.Entries))
	var exact, titles []*docs.Entry
	for _, entry := range found {
		if strings.EqualFold(entry.Title, term) {
			exact = append(exact, entry)
		}
		if strings.Contains(strings.ToLower(entry.Title), strings.ToLower(term)) {
			titles = append(titles, entry)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	if len(titles) > 0 {
		return titles
	}
	return found
}

func linkSelection(options [][]*docs.Entry, index *big.Int) []*docs.Entry {
	selected := make([]*docs.Entry, len(options))
	remaining := new(big.Int).Set(index)
	for i := len(options) - 1; i >= 0; i-- {
		quotient, digit := new(big.Int), new(big.Int)
		quotient.QuoRem(remaining, big.NewInt(int64(len(options[i]))), digit)
		selected[i] = options[i][int(digit.Int64())]
		remaining = quotient
	}
	return selected
}

func linkedMessage(query string, matches [][]int, selected []*docs.Entry) (string, string) {
	var message, description strings.Builder
	start := 0
	for i, match := range matches {
		entry := selected[i]
		message.WriteString(html.EscapeString(query[start:match[0]]))
		if i > 0 {
			description.WriteString(" · ")
		}
		if entry == nil {
			message.WriteString(html.EscapeString(query[match[0]:match[1]]))
			description.WriteString("No match: " + strings.TrimSpace(query[match[2]:match[3]]))
		} else {
			message.WriteString(`<a href="` + html.EscapeString(entry.URL) + `">`)
			message.WriteString(html.EscapeString(query[match[2]:match[3]]))
			message.WriteString("</a>")
			description.WriteString(entry.Title + " (" + entry.Library + ")")
		}
		start = match[1]
	}
	message.WriteString(html.EscapeString(query[start:]))
	return message.String(), description.String()
}

var issueNumber = regexp.MustCompile(`(?:^|\s)(nt)?#([0-9]+)\b`)

func issueResults(client *gotdbot.Client, query string) []gotdbot.InputInlineQueryResult {
	var results []gotdbot.InputInlineQueryResult
	seen := make(map[string]bool)
	for _, match := range issueNumber.FindAllStringSubmatch(query, -1) {
		repos := []string{"pytgcalls", "ntgcalls"}
		if match[1] == "nt" {
			repos = []string{"ntgcalls"}
		}
		for _, repo := range repos {
			id := repo + "_" + match[2]
			if seen[id] {
				continue
			}
			seen[id] = true
			title := fmt.Sprintf("[%s] Issue or PR #%s", repo, match[2])
			link := fmt.Sprintf("https://github.com/pytgcalls/%s/issues/%s", repo, match[2])
			formatted, err := client.GetFormattedText(`<a href="`+link+`">`+html.EscapeString(title)+`</a>`, nil, "HTML")
			if err != nil {
				continue
			}
			results = append(results, &gotdbot.InputInlineQueryResultArticle{
				Id: "issue_" + id, Title: title,
				InputMessageContent: &gotdbot.InputMessageText{Text: formatted},
			})
		}
	}
	return results
}
