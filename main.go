package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prosavage/github-contributions-chart-data/contributions"
)

type CacheEntry struct {
	data      []byte
	timestamp time.Time
}

func main() {
	rootLogger := log15.New("caller", "api")
	mux := http.NewServeMux()

	cache := make(map[string]CacheEntry)

	mux.HandleFunc("/contributions/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/contributions/") {
			username := strings.TrimPrefix(path, "/contributions/")
			reqLogger := rootLogger.New("username", username)
			cp := contributions.NewContributionsParser(reqLogger, username)

			if entry, ok := cache[username]; ok {
				if time.Since(entry.timestamp) < time.Hour {
					reqLogger.Info("Using cached data", "age", time.Since(entry.timestamp))
					_, err := w.Write(entry.data)
					if err != nil {
						reqLogger.Error("Failed to write response", "error", err)
						http.Error(w, "Failed to write response", http.StatusInternalServerError)
						return
					}
					return
				} else {
					reqLogger.Info("Cached data expired", "age", time.Since(entry.timestamp))
					delete(cache, username)
				}
			}

			startTime := time.Now()
			data, err := cp.ScrapeContributions()
			if err != nil {
				reqLogger.Error("Failed to scrape contributions", "error", err)
				http.Error(w, "Failed to scrape contributions", http.StatusInternalServerError)
				return
			}

			if len(cache) > 250 {
				reqLogger.Info("Cache size exceeded, clearing expired cache entries")
				for key, entry := range cache {
					if time.Since(entry.timestamp) > time.Hour {
						delete(cache, key)
					}
				}
				reqLogger.Info("Cache size after cleanup", "size", len(cache))
			}

			cache[username] = CacheEntry{
				data:      data,
				timestamp: time.Now(),
			}

			reqLogger.Info("Contributions scraped", "duration", time.Since(startTime))
			_, err = w.Write(data)
			if err != nil {
				reqLogger.Error("Failed to write response", "error", err)
				http.Error(w, "Failed to write response", http.StatusInternalServerError)
				return
			}

		} else {
			http.NotFound(w, r)
		}
	})

	http.ListenAndServe(":8080", mux)
}
