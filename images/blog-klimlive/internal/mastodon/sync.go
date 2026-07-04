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
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Skip translated posts — they share the same Mastodon toot as the default language
		if strings.Count(entry.Name(), ".") > 1 {
			continue
		}

		path := filepath.Join(postsDir, entry.Name())
		if err := syncPost(client, path, blogBaseURL); err != nil {
			log.Printf("skip %s: %v", entry.Name(), err)
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

	slug := strings.TrimSuffix(filepath.Base(path), ".md")
	postURL := fmt.Sprintf("%s/posts/%s/", strings.TrimRight(blogBaseURL, "/"), slug)

	tootText := composeToot(fm.Title, fm.Summary, postURL)

	log.Printf("posting to mastodon: %s", fm.Title)
	tootID, err := client.PostStatus(tootText, "public")
	if err != nil {
		return fmt.Errorf("post to mastodon: %w", err)
	}

	log.Printf("posted %s → toot %s", fm.Title, tootID)

	fm.MastodonPostID = tootID
	updatedFM, err := yaml.Marshal(&fm)
	if err != nil {
		return fmt.Errorf("marshal frontmatter: %w", err)
	}

	updated := fmt.Sprintf("---\n%s---\n%s", string(updatedFM), string(matches[2]))
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
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
