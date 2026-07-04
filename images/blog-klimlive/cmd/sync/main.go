package main

import (
	"log"
	"os"

	"github.com/frzifus/blog-klimlive.de/internal/mastodon"
)

func main() {
	instanceURL := requireEnv("MASTODON_INSTANCE_URL")
	accessToken := requireEnv("MASTODON_ACCESS_TOKEN")
	contentDir := envOr("CONTENT_DIR", "hugo/content")
	blogBaseURL := envOr("BLOG_BASE_URL", "https://blog.klimlive.de")

	client := mastodon.NewClient(instanceURL, accessToken)

	if err := mastodon.Sync(client, contentDir, blogBaseURL); err != nil {
		log.Fatalf("sync failed: %v", err)
	}

	log.Println("sync complete")
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
