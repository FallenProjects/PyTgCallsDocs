package main

//go:generate go run github.com/AshokShau/gotdbot/scripts/tools

import (
	"ashokshau/pytgdocs/internal/docs"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	ref := flag.String("ref", "dev", "docsdata branch or tag")
	configPath := flag.String("config", "", "config.xml URL or file")
	mapPath := flag.String("map", "", "map.json URL or file")
	out := flag.String("out", "docs.json", "output file")
	flag.Parse()
	if *configPath == "" {
		*configPath = fmt.Sprintf("https://raw.githubusercontent.com/pytgcalls/docsdata/%s/config.xml", *ref)
	}
	if *mapPath == "" {
		*mapPath = fmt.Sprintf("https://raw.githubusercontent.com/pytgcalls/docsdata/%s/map.json", *ref)
	}
	if err := generate(*ref, *configPath, *mapPath, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(ref, configPath, mapPath, out string) error {
	configData, err := readSource(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	mapData, err := readSource(mapPath)
	if err != nil {
		return fmt.Errorf("read map: %w", err)
	}
	config, err := parseConfig(configData)
	if err != nil {
		return err
	}
	var pages map[string]string
	if err := json.Unmarshal(mapData, &pages); err != nil {
		return fmt.Errorf("decode map: %w", err)
	}
	if len(pages) == 0 {
		return errors.New("map contains no pages")
	}
	paths := make([]string, 0, len(pages))
	for path := range pages {
		if strings.HasSuffix(path, ".xml") {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	catalog := &docs.Catalog{Ref: ref}
	for _, path := range paths {
		root, err := parseXML([]byte(pages[path]))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if root.Name != "page" {
			return fmt.Errorf("%s: expected page root", path)
		}
		if err := expand(root, config, nil); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		entry, extra, err := parsePage(path, root, ref)
		if err != nil {
			return err
		}
		catalog.Entries = append(catalog.Entries, entry)
		catalog.Entries = append(catalog.Entries, extra...)
	}
	sort.Slice(catalog.Entries, func(i, j int) bool {
		return catalog.Entries[i].Path < catalog.Entries[j].Path
	})
	if err := catalog.Validate(); err != nil {
		return err
	}
	return writeCatalog(out, catalog)
}

func readSource(source string) ([]byte, error) {
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		return os.ReadFile(source)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(source)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", source, response.Status)
	}
	return io.ReadAll(io.LimitReader(response.Body, 16<<20))
}

func writeCatalog(path string, catalog *docs.Catalog) error {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(catalog); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".docs-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(output.Bytes()); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
