package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prosavage/github-contributions-chart-data/contributions"
)

func main() {
	rootLogger := log15.New("caller", "api")
	mux := http.NewServeMux()

	mux.HandleFunc("/contributions/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/contributions/") {
			username := strings.TrimPrefix(path, "/contributions/")
			reqLogger := rootLogger.New("username", username)
			cp := contributions.NewContributionsParser(reqLogger, username)

			startTime := time.Now()
			data, err := cp.ScrapeContributions()
			if err != nil {
				reqLogger.Error("Failed to scrape contributions", "error", err)
				http.Error(w, "Failed to scrape contributions", http.StatusInternalServerError)
				return
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
