package mastodon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type postFrontmatter struct {
	Title          string `yaml:"title"`
	Date           string `yaml:"date"`
	Draft          bool   `yaml:"draft"`
	Summary        string `yaml:"summary"`
	MastodonPostID string `yaml:"mastodonPostId"`
}

var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.+?)\n---\n(.*)$`)

func Sync(client *Client, contentDir, blogBaseURL string) error {
	postsDir := filepath.Join(contentDir, "posts")

	entries, err := os.ReadDir(postsDir)
	if err != nil {
		return fmt.Errorf("read posts dir: %w", err)
	}

	for _, entry := range entries {
		var path string
		var name string

		if entry.IsDir() {
			// Page bundle: look for index.md inside the directory
			candidate := filepath.Join(postsDir, entry.Name(), "index.md")
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
			path = candidate
			name = entry.Name()
		} else if strings.HasSuffix(entry.Name(), ".md") {
			// Skip translated posts
			if strings.Count(entry.Name(), ".") > 1 {
				continue
			}
			path = filepath.Join(postsDir, entry.Name())
			name = entry.Name()
		} else {
			continue
		}

		if err := syncPost(client, path, blogBaseURL); err != nil {
			log.Printf("skip %s: %v", name, err)
		}
	}

	return nil
}

func syncPost(client *Client, path, blogBaseURL string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	matches := frontmatterRe.FindSubmatch(raw)
	if matches == nil {
		return fmt.Errorf("no frontmatter found")
	}

	var fm postFrontmatter
	if err := yaml.Unmarshal(matches[1], &fm); err != nil {
		return fmt.Errorf("parse frontmatter: %w", err)
	}

	if fm.Draft {
		return nil
	}
	if fm.MastodonPostID != "" {
		return nil
	}

	base := filepath.Base(path)
	slug := strings.TrimSuffix(base, ".md")
	if slug == "index" {
		slug = filepath.Base(filepath.Dir(path))
	}
	postURL := fmt.Sprintf("%s/posts/%s/", strings.TrimRight(blogBaseURL, "/"), slug)

	existingID, err := client.FindStatusByURL(postURL)
	if err != nil {
		log.Printf("warning: could not check existing posts: %v", err)
	}

	var tootID string
	if existingID != "" {
		log.Printf("already posted %s → toot %s", fm.Title, existingID)
		tootID = existingID
	} else {
		tootText := composeToot(fm.Title, fm.Summary, postURL)
		log.Printf("posting to mastodon: %s", fm.Title)
		tootID, err = client.PostStatus(tootText, "public")
		if err != nil {
			return fmt.Errorf("post to mastodon: %w", err)
		}
	}

	log.Printf("posted %s → toot %s", fm.Title, tootID)

	if err := setMastodonPostID(path, tootID); err != nil {
		return err
	}

	// Propagate to translations in the same directory
	dir := filepath.Dir(path)
	translations, _ := filepath.Glob(filepath.Join(dir, "index.*.md"))
	for _, t := range translations {
		if err := setMastodonPostID(t, tootID); err != nil {
			log.Printf("warning: could not update translation %s: %v", filepath.Base(t), err)
		} else {
			log.Printf("updated translation %s with toot %s", filepath.Base(t), tootID)
		}
	}

	return nil
}

func setMastodonPostID(path, tootID string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	matches := frontmatterRe.FindSubmatch(raw)
	if matches == nil {
		return fmt.Errorf("no frontmatter found in %s", path)
	}

	fmStr := string(matches[1])
	fmStr = strings.Replace(fmStr, `mastodonPostId: ""`, fmt.Sprintf("mastodonPostId: \"%s\"", tootID), 1)

	updated := fmt.Sprintf("---\n%s\n---\n%s", fmStr, string(matches[2]))
	return os.WriteFile(path, []byte(updated), 0644)
}

func composeToot(title, summary, url string) string {
	var text string
	if summary != "" {
		text = fmt.Sprintf("%s\n\n%s\n\n%s", title, summary, url)
	} else {
		text = fmt.Sprintf("%s\n\n%s", title, url)
	}

	if len(text) > 500 {
		maxSummary := 500 - len(title) - len(url) - 6 // 6 for newlines
		if maxSummary > 3 {
			text = fmt.Sprintf("%s\n\n%s...\n\n%s", title, summary[:maxSummary], url)
		} else {
			text = fmt.Sprintf("%s\n\n%s", title, url)
		}
	}

	return text
}
