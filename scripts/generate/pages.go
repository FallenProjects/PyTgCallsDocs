package main

import (
	"ashokshau/pytgdocs/internal/docs"
	"fmt"
	"html"
	"net/url"
	"strings"
)

var languageNames = map[string]string{
	"python": "Python", "c": "C", "node": "Node.js", "java": "Java", "rust": "Rust",
	"pyrogram": "PyrogramMod", "telethon": "Telethon", "hydrogram": "Hydrogram",
	"x86": "x86_64", "arm": "ARM64", "shared": "Shared", "static": "Static",
}

func parsePage(path string, root *node, ref string) (*docs.Entry, []*docs.Entry, error) {
	r := renderer{ref: ref}
	heading := root.first("h1")
	if heading == nil {
		return nil, nil, fmt.Errorf("%s: missing h1", path)
	}
	title := plain(heading.Children)
	if title == "" {
		return nil, nil, fmt.Errorf("%s: empty h1", path)
	}
	library := ""
	switch {
	case strings.HasPrefix(path, "/NTgCalls/"):
		library = "NTgCalls"
	case strings.HasPrefix(path, "/PyTgCalls/"):
		library = "PyTgCalls"
	default:
		return nil, nil, fmt.Errorf("%s: unknown library", path)
	}
	entry := &docs.Entry{
		ID: docs.ID(path), Path: path, Title: title, Library: library,
		Kind: kindForPath(path), URL: r.link(strings.TrimSuffix(path, ".xml")),
	}
	if entry.Kind == "guide" && path != "/PyTgCalls/Changelogs.xml" {
		entry.Description = guideDescription(root, r)
	} else {
		entry.Description = pageDescription(root, r)
	}
	entry.Preview = preview(entry.Description)

	global := contentFor(root, r, entry.Kind != "guide")
	examples := make(map[string]*docs.Example)
	for _, child := range root.Children {
		if child.Node == nil || child.Node.Name != "multisyntax" {
			continue
		}
		for _, syntax := range child.Node.children("syntax-highlight") {
			id := syntax.attr("id")
			if id != "" {
				examples[id] = &docs.Example{Language: syntax.attr("language"), Code: dedent(plainRaw(syntax.Children))}
			}
		}
	}
	labels := make(map[string]string)
	for _, tabSet := range root.children("multisyntax") {
		for _, tabs := range tabSet.children("tabs") {
			for _, tab := range tabs.children("tab") {
				labels[tab.attr("id")] = plain(tab.Children)
			}
		}
	}
	positions := make(map[string]int)
	for _, block := range root.children("lang-block") {
		id := block.attr("language")
		if id == "" {
			return nil, nil, fmt.Errorf("%s: lang-block has no language", path)
		}
		content := contentFor(block, r, true)
		content.Example = examples[id]
		if position, exists := positions[id]; exists {
			mergeContent(&entry.Languages[position].Content, content)
			continue
		}
		label := labels[id]
		if label == "" {
			label = languageNames[id]
		}
		if label == "" {
			label = id
		}
		variant := docs.Language{ID: id, Label: label, Content: content}
		if block.attr("default") == "true" {
			entry.Languages = append([]docs.Language{variant}, entry.Languages...)
			for other := range positions {
				positions[other]++
			}
			positions[id] = 0
		} else {
			positions[id] = len(entry.Languages)
			entry.Languages = append(entry.Languages, variant)
		}
	}
	if len(entry.Languages) == 0 && (global.Signature != "" || global.Example != nil || len(global.Sections) > 0) {
		entry.Content = &global
	}

	var extra []*docs.Entry
	if path == "/PyTgCalls/Changelogs.xml" {
		extra = append(extra, changelogEntries(root, r)...)
	}
	if path == "/PyTgCalls/Examples.xml" {
		extra = append(extra, exampleEntries(root, r)...)
	}
	return entry, extra, nil
}

func kindForPath(path string) string {
	switch {
	case strings.Contains(path, " Methods/"):
		return "method"
	case strings.Contains(path, "/Callbacks/"):
		return "callback"
	case strings.Contains(path, "/Available Enums/"):
		return "enum"
	case strings.Contains(path, "/Available Structs/"):
		return "struct"
	case strings.Contains(path, "/Available Types/"), strings.Contains(path, "/Advanced Types/"):
		return "type"
	case strings.Contains(path, "/Stream Descriptors/"):
		return "descriptor"
	default:
		return "guide"
	}
}

func pageDescription(root *node, r renderer) string {
	var alert *node
	for _, child := range root.Children {
		if child.Node == nil {
			continue
		}
		switch child.Node.Name {
		case "text":
			if result := r.rich([]part{child}); result != "" {
				return result
			}
		case "subtext":
			if text := firstIntroText(child.Node); text != nil {
				return r.rich([]part{{Node: text}})
			}
		case "alert":
			if alert == nil {
				alert = child.Node
			}
		case "separator", "lang-block":
			if alert != nil {
				return r.rich([]part{{Node: alert}})
			}
			return ""
		}
	}
	for _, child := range root.Children {
		if child.Node == nil {
			continue
		}
		if child.Node.Name == "category" {
			if banner := child.Node.first("banner"); banner != nil {
				return html.EscapeString(strings.TrimSpace(banner.attr("description")))
			}
		}
	}
	return ""
}

func guideDescription(root *node, r renderer) string {
	var content []part
	titleSeen := false
	for index, child := range root.Children {
		if child.Node == nil {
			continue
		}
		name := child.Node.Name
		switch name {
		case "category", "lang-block", "multisyntax":
			continue
		case "h1":
			if !titleSeen {
				titleSeen = true
				continue
			}
		}
		if name == "h1" || name == "h2" || name == "h3" {
			for next := index + 1; next < len(root.Children); next++ {
				candidate := root.Children[next].Node
				if candidate == nil || candidate.Name == "br" || candidate.Name == "separator" {
					continue
				}
				if candidate.Name == "category" || candidate.Name == "lang-block" || candidate.Name == "multisyntax" {
					name = ""
				}
				break
			}
			if name == "" {
				continue
			}
		}
		if name == "separator" && (len(content) == 0 || content[len(content)-1].Node.Name == "separator") {
			continue
		}
		content = append(content, child)
	}
	for len(content) > 0 && (content[len(content)-1].Node.Name == "separator" || content[len(content)-1].Node.Name == "br") {
		content = content[:len(content)-1]
	}
	return r.rich(content)
}

func firstIntroText(root *node) *node {
	for _, child := range root.Children {
		if child.Node == nil {
			continue
		}
		if child.Node.Name == "pg-title" || child.Node.Name == "category" {
			return nil
		}
		if child.Node.Name == "text" {
			return child.Node
		}
		if child.Node.Name == "subtext" {
			if text := firstIntroText(child.Node); text != nil {
				return text
			}
		}
	}
	return nil
}

func preview(rich string) string {
	text := docs.Plain(rich)
	runes := []rune(text)
	if len(runes) > 220 {
		return string(runes[:217]) + "..."
	}
	return text
}

func contentFor(root *node, r renderer, includeSignature bool) docs.Content {
	var content docs.Content
	if root.Name == "lang-block" {
		var notes []string
		for _, text := range root.children("text") {
			if rendered := r.rich(text.Children); rendered != "" {
				notes = append(notes, rendered)
			}
		}
		content.Notes = strings.Join(notes, "<br><br>")
	}
	if includeSignature {
		if signature := root.first("category-title"); signature != nil {
			content.Signature = r.rich(signature.Children)
		}
	}
	if syntax := root.first("syntax-highlight"); syntax != nil {
		language := syntax.attr("language")
		if language == "" {
			language = "python"
		}
		content.Example = &docs.Example{Language: language, Code: dedent(plainRaw(syntax.Children))}
	}
	content.Sections = sectionsFor(root, r)
	return content
}

func mergeContent(target *docs.Content, extra docs.Content) {
	if target.Signature == "" {
		target.Signature = extra.Signature
	}
	if extra.Notes != "" {
		if target.Notes != "" {
			target.Notes += "<br><br>"
		}
		target.Notes += extra.Notes
	}
	if target.Example == nil {
		target.Example = extra.Example
	}
	target.Sections = append(target.Sections, extra.Sections...)
}

func sectionsFor(root *node, r renderer) []docs.Section {
	var sections []docs.Section
	var visit func(*node)
	visit = func(parent *node) {
		for i, child := range parent.Children {
			if child.Node == nil {
				continue
			}
			n := child.Node
			if n.Name == "pg-title" {
				title := plain(n.Children)
				var source []part
				for j := i + 1; j < len(parent.Children); j++ {
					next := parent.Children[j].Node
					if next == nil {
						continue
					}
					if next.Name == "pg-title" {
						break
					}
					if next.Name == "subtext" {
						source = next.Children
						break
					}
				}
				if items := parseItems(source, r); len(items) > 0 {
					sections = append(sections, docs.Section{Title: title, Items: items})
				}
				continue
			}
			if n.Name != "syntax-highlight" && n.Name != "multisyntax" && n.Name != "lang-block" && n.Name != "table" {
				visit(n)
			}
		}
	}
	visit(root)
	return sections
}

func parseItems(parts []part, r renderer) []docs.Item {
	var items []docs.Item
	var current *docs.Item
	flush := func() {
		if current != nil && (current.Name != "" || current.Description != "") {
			current.Description = strings.TrimPrefix(strings.TrimSuffix(current.Description, "<br>"), "<br>")
			items = append(items, *current)
		}
		current = nil
	}
	for _, element := range parts {
		if element.Node == nil {
			continue
		}
		n := element.Node
		switch n.Name {
		case "category-title":
			flush()
			current = &docs.Item{Name: r.rich(n.Children)}
		case "text", "alert", "list", "item", "subtext":
			text := r.rich([]part{{Node: n}})
			if text == "" {
				continue
			}
			if current == nil {
				current = &docs.Item{}
			}
			if current.Description != "" {
				current.Description += "<br>"
			}
			current.Description += text
		}
	}
	flush()
	return items
}

func changelogEntries(root *node, r renderer) []*docs.Entry {
	var entries []*docs.Entry
	versions := make(map[string]int)
	for _, category := range root.children("category") {
		banner := category.first("banner")
		if banner == nil || banner.attr("version") == "" {
			continue
		}
		version := banner.attr("version")
		versions[version]++
		name := version
		if versions[version] > 1 {
			name = fmt.Sprintf("%s-%d", version, versions[version])
		}
		path := "/PyTgCalls/Changelogs/" + name + ".xml"
		description := strings.ReplaceAll(html.EscapeString(strings.TrimSpace(banner.attr("description"))), "\n", "<br>")
		for _, subtext := range category.children("subtext") {
			if detail := strings.TrimPrefix(strings.TrimSuffix(r.rich(subtext.Children), "<br>"), "<br>"); detail != "" {
				if description != "" {
					description += "<br><br>"
				}
				description += detail
			}
		}
		url := r.link("/PyTgCalls/Changelogs#" + strings.ReplaceAll(version, ".", ""))
		entries = append(entries, &docs.Entry{
			ID: docs.ID(path), Path: path, Title: "Changelog " + version + ": " + banner.attr("bigtitle"),
			Library: "PyTgCalls", Kind: "changelog", Description: description, Preview: preview(description), URL: url,
		})
	}
	return entries
}

func exampleEntries(root *node, r renderer) []*docs.Entry {
	var entries []*docs.Entry
	for _, table := range root.children("table") {
		for _, row := range table.children("item") {
			columns := row.children("column")
			if len(columns) < 2 {
				continue
			}
			link := columns[0].first("ref-shi")
			if link == nil {
				continue
			}
			name := plain(link.Children)
			path := "/PyTgCalls/Examples/" + name + ".xml"
			source := link.attr("url")
			if !strings.HasPrefix(source, "http") {
				source = "https://github.com/pytgcalls/pytgcalls/tree/master/" + source
			}
			if parsed, err := url.Parse(source); err == nil {
				source = parsed.String()
			}
			description := r.rich(columns[1].Children)
			entries = append(entries, &docs.Entry{
				ID: docs.ID(path), Path: path, Title: "Example: " + name, Library: "PyTgCalls",
				Kind: "example", Description: description, Preview: preview(description), URL: source,
			})
		}
	}
	return entries
}
