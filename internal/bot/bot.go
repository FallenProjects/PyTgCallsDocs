package bot

import (
	"ashokshau/pytgdocs/internal/docs"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AshokShau/gotdbot"
)

type Bot struct {
	Client  *gotdbot.Client
	Catalog *docs.Catalog
	Started time.Time
}

func New(client *gotdbot.Client, catalog *docs.Catalog) *Bot {
	return &Bot{Client: client, Catalog: catalog, Started: time.Now()}
}

func (b *Bot) Register() {
	b.Client.OnCommand("start", b.start)
	b.Client.OnCommand("ping", b.ping)
	b.Client.OnUpdateNewInlineQuery(b.inlineQuery, nil)
	b.Client.OnUpdateNewInlineCallbackQuery(b.callback, nil)
}

func (b *Bot) start(client *gotdbot.Client, message *gotdbot.Message) error {
	username := client.Me().Usernames.EditableUsername
	if message.Args() == "inline_help" {
		text := fmt.Sprintf("<b>Link documentation in a message</b>\n\nWrap a page name in plus signs: <code>@%s Read +Quick Start+ before using +Play+.</code>\n\nChoose a result to insert the message with links.", html.EscapeString(username))
		_, err := message.ReplyText(client, text, &gotdbot.SendMessageOpts{ParseMode: gotdbot.ParseModeHTML})
		return err
	}
	text := fmt.Sprintf("<b>PyTgCalls documentation</b>\n\nSearch with <code>@%s play</code> or insert a linked message with <code>@%s Try +Play+.</code>\n\nUse <code>#123</code> for a PyTgCalls issue or <code>nt#123</code> for an NTgCalls issue.\n\n<a href=\"https://pytgcalls.github.io/\">Open the full documentation</a>", html.EscapeString(username), html.EscapeString(username))
	_, err := message.ReplyText(client, text, &gotdbot.SendMessageOpts{ParseMode: gotdbot.ParseModeHTML})
	return err
}

func (b *Bot) ping(client *gotdbot.Client, message *gotdbot.Message) error {
	uptime := time.Since(b.Started).Round(time.Second)
	_, err := message.ReplyText(client, fmt.Sprintf("Pong · uptime %s", uptime), nil)
	return err
}

func (b *Bot) inlineQuery(client *gotdbot.Client, query *gotdbot.UpdateNewInlineQuery) error {
	q := strings.TrimSpace(query.Query)
	if linkedTerm.MatchString(q) {
		var additional []gotdbot.InputInlineQueryResult
		if strings.Contains(q, "#") {
			additional = issueResults(client, q)
		}
		results, next := b.linkedResults(client, q, query.Offset, additional)
		return client.AnswerInlineQuery(query.Id, results, &gotdbot.AnswerInlineQueryOpts{
			CacheTime:  300,
			NextOffset: next,
			Button: &gotdbot.InlineQueryResultsButton{
				Text: "How to link pages",
				Type: &gotdbot.InlineQueryResultsButtonTypeStartBot{Parameter: "inline_help"},
			},
		})
	}
	var special []gotdbot.InputInlineQueryResult
	if strings.Contains(q, "#") {
		special = append(special, issueResults(client, q)...)
	}
	special = append(special, shortcutResults(client, q)...)
	entries := b.Catalog.Search(q, len(b.Catalog.Entries))
	if q == "" {
		entries = browseEntries(b.Catalog.Entries)
	}
	start, end, next := inlinePage(len(special)+len(entries), query.Offset)
	results := make([]gotdbot.InputInlineQueryResult, 0, end-start)
	for index := start; index < end && index < len(special); index++ {
		results = append(results, special[index])
	}
	for index := max(start, len(special)); index < end; index++ {
		entry := entries[index-len(special)]
		results = append(results, &gotdbot.InputInlineQueryResultArticle{
			Id:                  "doc_" + entry.ID,
			Title:               fmt.Sprintf("[%s] %s", entry.Library, entry.Title),
			Description:         entry.Preview,
			InputMessageContent: richMessage(renderEntry(entry, "")),
			ReplyMarkup:         entryKeyboard(entry, "main", ""),
		})
	}
	opts := &gotdbot.AnswerInlineQueryOpts{CacheTime: 300, NextOffset: next}
	if strings.Contains(q, "+") {
		opts.Button = &gotdbot.InlineQueryResultsButton{
			Text: "How to link pages",
			Type: &gotdbot.InlineQueryResultsButtonTypeStartBot{Parameter: "inline_help"},
		}
	}
	return client.AnswerInlineQuery(query.Id, results, opts)
}

func browseEntries(entries []*docs.Entry) []*docs.Entry {
	browsed := append([]*docs.Entry(nil), entries...)
	rank := func(entry *docs.Entry) int {
		switch entry.Title {
		case "Quick Start":
			return 0
		case "Install Guide":
			return 1
		case "Changelogs":
			return 2
		case "Examples":
			return 3
		}
		if entry.Kind == "guide" {
			return 4
		}
		return 5
	}
	sort.Slice(browsed, func(i, j int) bool {
		a, b := browsed[i], browsed[j]
		if rank(a) != rank(b) {
			return rank(a) < rank(b)
		}
		if a.Title != b.Title {
			return a.Title < b.Title
		}
		return a.Path < b.Path
	})
	return browsed
}

func inlinePage(total int, offset string) (start, end int, next string) {
	start, err := strconv.Atoi(offset)
	if err != nil || start < 0 || start >= total {
		start = 0
	}
	end = min(start+40, total)
	if end < total {
		next = strconv.Itoa(end)
	}
	return start, end, next
}

var callbackData = regexp.MustCompile(`^(main|example|s[0-9]+):([0-9a-f]{16})(?::([a-z0-9_]+))?$`)

func richMessage(text string) *gotdbot.InputMessageRichMessage {
	return &gotdbot.InputMessageRichMessage{
		Message: &gotdbot.InputRichMessage{
			Source: &gotdbot.RichMessageSourceHtml{Text: text},
		},
	}
}

func (b *Bot) callback(client *gotdbot.Client, query *gotdbot.UpdateNewInlineCallbackQuery) error {
	payload, ok := query.Payload.(*gotdbot.CallbackQueryPayloadData)
	if !ok {
		return client.AnswerCallbackQuery(query.Id, &gotdbot.AnswerCallbackQueryOpts{Text: "Invalid action"})
	}
	match := callbackData.FindStringSubmatch(string(payload.Data))
	if match == nil {
		return client.AnswerCallbackQuery(query.Id, &gotdbot.AnswerCallbackQueryOpts{Text: "Invalid action"})
	}
	entry := b.Catalog.ByID(match[2])
	if entry == nil {
		return client.AnswerCallbackQuery(query.Id, &gotdbot.AnswerCallbackQueryOpts{Text: "Page unavailable"})
	}
	view, language := match[1], match[3]
	text := renderView(entry, view, language)
	if text == "" {
		return client.AnswerCallbackQuery(query.Id, &gotdbot.AnswerCallbackQueryOpts{Text: "Section unavailable"})
	}
	err := client.EditInlineMessageText(query.InlineMessageId, richMessage(text), &gotdbot.EditInlineMessageTextOpts{ReplyMarkup: entryKeyboard(entry, view, language)})
	if err != nil && !strings.Contains(err.Error(), "MESSAGE_NOT_MODIFIED") {
		slog.Error("edit inline page", "path", entry.Path, "view", view, "error", err)
		return client.AnswerCallbackQuery(query.Id, &gotdbot.AnswerCallbackQueryOpts{Text: "Unable to update this page"})
	}
	return client.AnswerCallbackQuery(query.Id, nil)
}
