package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/url"
	"strings"
)

type part struct {
	Text string
	Node *node
}

type node struct {
	Name     string
	Attrs    map[string]string
	Children []part
}

func (n *node) attr(name string) string { return n.Attrs[name] }

func (n *node) children(name string) []*node {
	var found []*node
	for _, child := range n.Children {
		if child.Node != nil && child.Node.Name == name {
			found = append(found, child.Node)
		}
	}
	return found
}

func (n *node) first(name string) *node {
	for _, child := range n.Children {
		if child.Node != nil && child.Node.Name == name {
			return child.Node
		}
	}
	return nil
}

func parseXML(data []byte) (*node, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *node
	var stack []*node
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			current := &node{Name: value.Name.Local, Attrs: make(map[string]string)}
			for _, attr := range value.Attr {
				current.Attrs[attr.Name.Local] = attr.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("multiple XML roots")
				}
				root = current
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, part{Node: current})
			}
			stack = append(stack, current)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				current := stack[len(stack)-1]
				current.Children = append(current.Children, part{Text: string(value)})
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("empty XML")
	}
	return root, nil
}

func parseConfig(data []byte) (map[string]*node, error) {
	root, err := parseXML(data)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if root.Name != "config" {
		return nil, fmt.Errorf("config: expected config root")
	}
	options := make(map[string]*node)
	for _, option := range root.children("option") {
		id := option.attr("id")
		if id == "" || options[id] != nil {
			return nil, fmt.Errorf("config: missing or duplicate option id %q", id)
		}
		options[id] = option
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("config: no options")
	}
	return options, nil
}

func expand(root *node, options map[string]*node, active map[string]bool) error {
	if active == nil {
		active = make(map[string]bool)
	}
	var expanded []part
	for _, child := range root.Children {
		if child.Node == nil {
			expanded = append(expanded, child)
			continue
		}
		if child.Node.Name == "config" {
			id := child.Node.attr("id")
			option := options[id]
			if option == nil {
				return fmt.Errorf("unknown config option %q", id)
			}
			if active[id] {
				return fmt.Errorf("cyclic config option %q", id)
			}
			active[id] = true
			copy := clone(option)
			if err := expand(copy, options, active); err != nil {
				return fmt.Errorf("config option %q: %w", id, err)
			}
			delete(active, id)
			expanded = append(expanded, copy.Children...)
			continue
		}
		if err := expand(child.Node, options, active); err != nil {
			return err
		}
		expanded = append(expanded, child)
	}
	root.Children = expanded
	return nil
}

func clone(source *node) *node {
	copy := &node{Name: source.Name, Attrs: source.Attrs}
	for _, child := range source.Children {
		if child.Node != nil {
			copy.Children = append(copy.Children, part{Node: clone(child.Node)})
		} else {
			copy.Children = append(copy.Children, child)
		}
	}
	return copy
}

func plain(parts []part) string {
	var text strings.Builder
	var walk func([]part)
	walk = func(parts []part) {
		for _, child := range parts {
			if child.Node != nil {
				walk(child.Node.Children)
			} else {
				text.WriteString(child.Text)
			}
		}
	}
	walk(parts)
	return strings.Join(strings.Fields(text.String()), " ")
}

type renderer struct{ ref string }

func (r renderer) rich(parts []part) string {
	var out strings.Builder
	for _, piece := range parts {
		if piece.Node == nil {
			text := strings.Join(strings.Fields(piece.Text), " ")
			if text == "" {
				if piece.Text != "" {
					out.WriteByte(' ')
				}
				continue
			}
			if len(piece.Text) > 0 && (piece.Text[0] == ' ' || piece.Text[0] == '\n' || piece.Text[0] == '\t') {
				out.WriteByte(' ')
			}
			out.WriteString(html.EscapeString(text))
			if len(piece.Text) > 0 && (piece.Text[len(piece.Text)-1] == ' ' || piece.Text[len(piece.Text)-1] == '\n' || piece.Text[len(piece.Text)-1] == '\t') {
				out.WriteByte(' ')
			}
			continue
		}
		n := piece.Node
		if n.Name == "syntax-highlight" {
			codeTag := "<code>"
			if language := n.attr("language"); language != "" {
				codeTag = `<code class="language-` + html.EscapeString(language) + `">`
			}
			out.WriteString("<pre>" + codeTag + html.EscapeString(dedent(plainRaw(n.Children))) + "</code></pre>")
			continue
		}
		if n.Name == "multisyntax" {
			if n.attr("as-blame") == "true" {
				snippets := n.children("syntax-highlight")
				out.WriteString("<br><pre><code class=\"language-diff\">" + html.EscapeString(codeDiff(dedent(plainRaw(snippets[0].Children)), dedent(plainRaw(snippets[1].Children)))) + "</code></pre><br>")
			}
			continue
		}
		if n.Name == "table" {
			out.WriteString("<table bordered striped compact>")
			if header := n.first("definitions"); header != nil {
				out.WriteString("<tr>")
				for _, column := range header.children("column") {
					out.WriteString("<th>" + r.rich(column.Children) + "</th>")
				}
				out.WriteString("</tr>")
			}
			for _, row := range n.children("item") {
				out.WriteString("<tr>")
				for _, column := range row.children("column") {
					out.WriteString("<td>" + r.rich(column.Children) + "</td>")
				}
				out.WriteString("</tr>")
			}
			out.WriteString("</table>")
			continue
		}
		inside := r.rich(n.Children)
		switch n.Name {
		case "b", "sb", "strong":
			out.WriteString("<b>" + inside + "</b>")
		case "i", "em":
			out.WriteString("<i>" + inside + "</i>")
		case "u", "ins":
			out.WriteString("<u>" + inside + "</u>")
		case "shi", "shi-inline", "code":
			out.WriteString("<code>" + inside + "</code>")
		case "a":
			out.WriteString(`<a href="` + html.EscapeString(r.link(n.attr("href"))) + `">` + inside + `</a>`)
		case "docs-ref":
			out.WriteString(`<a href="` + html.EscapeString(r.link(n.attr("link"))) + `">` + inside + `</a>`)
		case "ref-shi":
			link := n.attr("url")
			if !strings.HasPrefix(link, "http") {
				link = "https://github.com/pytgcalls/pytgcalls/tree/master/" + link
			}
			out.WriteString(`<a href="` + html.EscapeString(link) + `">` + inside + `</a>`)
		case "github-ref":
			user, repo := n.attr("user"), n.attr("reponame")
			link := "https://github.com/" + url.PathEscape(user) + "/" + url.PathEscape(repo)
			out.WriteString(`<a href="` + link + `">` + html.EscapeString(user+"/"+repo) + `</a>`)
		case "br":
			out.WriteString("<br>")
		case "h1", "h2":
			out.WriteString("<h2>" + strings.TrimSpace(inside) + "</h2>")
		case "list":
			tag := "ul"
			if n.attr("style") == "numbers" {
				tag = "ol"
			}
			out.WriteString("<" + tag + ">" + strings.TrimSpace(inside) + "</" + tag + ">")
		case "item":
			if len(n.children("item")) > 0 {
				out.WriteString(strings.TrimSpace(inside))
			} else {
				out.WriteString("<li>" + strings.TrimSpace(inside) + "</li>")
			}
		case "h3":
			out.WriteString("<h3>" + strings.TrimSpace(inside) + "</h3>")
		case "alert":
			out.WriteString("<blockquote><b>" + html.EscapeString(strings.ToUpper(n.attr("type"))) + "</b><br>" + strings.TrimSpace(inside) + "</blockquote>")
		case "separator":
			out.WriteString("<hr/>")
		case "tabs", "tab", "lang-tabs", "banner", "p2p-banner":
		default:
			out.WriteString(inside)
		}
	}
	return strings.TrimSpace(strings.ReplaceAll(out.String(), "<br> <br>", "<br><br>"))
}

func plainRaw(parts []part) string {
	var out strings.Builder
	for _, part := range parts {
		if part.Node != nil {
			out.WriteString(plainRaw(part.Node.Children))
		} else {
			out.WriteString(part.Text)
		}
	}
	return out.String()
}

func codeDiff(before, after string) string {
	oldLines := strings.Split(before, "\n")
	newLines := strings.Split(after, "\n")
	common := make([][]int, len(oldLines)+1)
	for i := range common {
		common[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				common[i][j] = common[i+1][j+1] + 1
			} else {
				common[i][j] = max(common[i+1][j], common[i][j+1])
			}
		}
	}
	var lines []string
	for i, j := 0, 0; i < len(oldLines) || j < len(newLines); {
		switch {
		case i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j]:
			lines = append(lines, "  "+oldLines[i])
			i++
			j++
		case i < len(oldLines) && (j == len(newLines) || common[i+1][j] >= common[i][j+1]):
			lines = append(lines, "- "+oldLines[i])
			i++
		default:
			lines = append(lines, "+ "+newLines[j])
			j++
		}
	}
	return strings.Join(lines, "\n")
}

func dedent(text string) string {
	lines := strings.Split(strings.TrimRight(strings.TrimLeft(text, "\n"), " \t\r\n"), "\n")
	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		spaces := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < 0 || spaces < indent {
			indent = spaces
		}
	}
	if indent < 0 {
		return ""
	}
	for i, line := range lines {
		if len(line) >= indent {
			lines[i] = line[indent:]
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func (r renderer) link(raw string) string {
	if strings.HasPrefix(raw, "/") {
		raw = "https://pytgcalls.github.io" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host != "pytgcalls.github.io" {
		return raw
	}
	if r.ref == "master" {
		query := u.Query()
		query.Del("ref")
		u.RawQuery = query.Encode()
	} else {
		query := u.Query()
		query.Set("ref", r.ref)
		u.RawQuery = query.Encode()
	}
	return u.String()
}
