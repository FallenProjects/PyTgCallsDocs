package bot

import (
	"ashokshau/pytgdocs/internal/docs"
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/AshokShau/gotdbot"
)

func renderView(entry *docs.Entry, view, language string) string {
	content, _ := entry.Selected(language)
	switch view {
	case "main":
		return renderEntry(entry, language)
	case "example":
		if content.Example != nil {
			return renderExample(entry, content.Example)
		}
	default:
		if strings.HasPrefix(view, "s") {
			index, err := strconv.Atoi(view[1:])
			if err == nil && index >= 0 && index < len(content.Sections) {
				return renderSection(entry, content.Sections[index])
			}
		}
	}
	return ""
}

func renderEntry(entry *docs.Entry, language string) string {
	content, _ := entry.Selected(language)
	var text strings.Builder
	text.WriteString("<b>" + html.EscapeString(entry.Title) + "</b> · " + html.EscapeString(entry.Library))
	if entry.Kind != "guide" {
		text.WriteString(" " + html.EscapeString(entry.Kind))
	}
	if entry.Description != "" {
		if entry.Kind == "changelog" || entry.Kind == "guide" && utf8.RuneCountInString(docs.Plain(entry.Description)) > 700 {
			text.WriteString("<br><br><details><summary>Full " + html.EscapeString(entry.Kind) + "</summary>" + entry.Description + "</details>")
		} else {
			text.WriteString("<br><br>" + entry.Description)
		}
	}
	if content.Signature != "" {
		text.WriteString("<br><br>" + content.Signature)
	}
	if content.Notes != "" {
		text.WriteString("<br><br>" + content.Notes)
	}
	text.WriteString("<br><br>" + onlineLink(entry))
	return text.String()
}

func renderExample(entry *docs.Entry, example *docs.Example) string {
	return fmt.Sprintf("<b>%s · example</b><br><br><pre><code class=\"language-%s\">%s</code></pre><br><br>%s",
		html.EscapeString(entry.Title), html.EscapeString(example.Language), html.EscapeString(example.Code), onlineLink(entry))
}

func renderSection(entry *docs.Entry, section docs.Section) string {
	var body strings.Builder
	body.WriteString("<ul>")
	for _, item := range section.Items {
		name := strings.TrimSpace(item.Name)
		description := strings.TrimSpace(item.Description)
		if name == "" && description == "" {
			continue
		}
		body.WriteString("<li>")
		if name != "" {
			body.WriteString(name)
		}
		if description != "" {
			if name != "" {
				if strings.EqualFold(section.Title, "PARAMETERS") {
					body.WriteString(": ")
				} else {
					body.WriteString("<br>")
				}
			}
			body.WriteString(description)
		}
		body.WriteString("</li>")
	}
	body.WriteString("</ul>")
	var text strings.Builder
	text.WriteString("<b>" + html.EscapeString(entry.Title) + " · " + html.EscapeString(section.Title) + "</b><br><br>")
	if utf8.RuneCountInString(docs.Plain(body.String())) > 700 {
		text.WriteString("<details><summary>Full " + html.EscapeString(strings.ToLower(section.Title)) + "</summary>" + body.String() + "</details>")
	} else {
		text.WriteString(body.String())
	}
	text.WriteString("<br><br>" + onlineLink(entry))
	return text.String()
}

func onlineLink(entry *docs.Entry) string {
	return `<a href="` + html.EscapeString(entry.URL) + `">View online</a>`
}

func entryKeyboard(entry *docs.Entry, view, language string) *gotdbot.ReplyMarkupInlineKeyboard {
	content, selected := entry.Selected(language)
	keyboard := &gotdbot.ReplyMarkupInlineKeyboard{}
	if len(entry.Languages) > 1 {
		var row []gotdbot.InlineKeyboardButton
		for _, variant := range entry.Languages {
			label := variant.Label
			if variant.ID == selected {
				label = "• " + label
			}
			targetView := view
			if strings.HasPrefix(view, "s") {
				index, _ := strconv.Atoi(view[1:])
				if index >= len(variant.Content.Sections) {
					targetView = "main"
				}
			}
			row = append(row, callbackButton(label, targetView, entry.ID, variant.ID))
		}
		keyboard.Rows = append(keyboard.Rows, row)
	}
	var buttons []gotdbot.InlineKeyboardButton
	if view != "main" {
		buttons = append(buttons, callbackButton("Overview", "main", entry.ID, selected))
	}
	if content.Example != nil && view != "example" {
		buttons = append(buttons, callbackButton("Example", "example", entry.ID, selected))
	}
	for index, section := range content.Sections {
		sectionView := fmt.Sprintf("s%d", index)
		if view == sectionView {
			continue
		}
		label := strings.TrimSpace(section.Title)
		if label == "" {
			label = "Details"
		}
		if utf8.RuneCountInString(label) > 24 {
			label = string([]rune(label)[:24]) + "…"
		}
		buttons = append(buttons, callbackButton(label, sectionView, entry.ID, selected))
	}
	buttons = append(buttons, gotdbot.InlineKeyboardButton{Text: "🌐 Docs", Type: &gotdbot.InlineKeyboardButtonTypeUrl{Url: entry.URL}})
	for len(buttons) > 0 {
		count := min(2, len(buttons))
		keyboard.Rows = append(keyboard.Rows, buttons[:count])
		buttons = buttons[count:]
	}
	return keyboard
}

func callbackButton(label, view, id, language string) gotdbot.InlineKeyboardButton {
	data := view + ":" + id
	if language != "" {
		data += ":" + language
	}
	return gotdbot.InlineKeyboardButton{Text: label, Type: &gotdbot.InlineKeyboardButtonTypeCallback{Data: []byte(data)}}
}
