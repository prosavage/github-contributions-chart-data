package contributions

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
)

type Day struct {
	Date  time.Time `json:"date"`
	Level int       `json:"level"`
	Count int       `json:"count"`
}

func (d Day) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Date  string `json:"date"`
		Level int    `json:"level"`
		Count int    `json:"count"`
	}{
		Date:  d.Date.Format("2006-01-02"), // Format the date as "YYYY-MM-DD"
		Level: d.Level,
		Count: d.Count,
	})
}

type ContributionsParser struct {
	logger   log15.Logger
	username string
}

func NewContributionsParser(logger log15.Logger, username string) *ContributionsParser {
	return &ContributionsParser{
		logger:   logger,
		username: username,
	}
}

func (cp *ContributionsParser) ScrapeContributions() ([]byte, error) {
	years, err := cp.scrapeYears()
	if err != nil {
		return []byte{}, errors.Wrap(err, "scrape years")
	}

	cp.logger.Info("Found years", "years", years)

	contributionData := make(map[string][]Day)
	totalContributionsData := make(map[string]int)

	var wg sync.WaitGroup

	for _, year := range years {
		wg.Add(1)
		go func(year int) {
			defer wg.Done()
			total, daysForYear, err := cp.scrapeYearData(year, false)
			if err != nil {
				cp.logger.Error("Failed to scrape year data", "error", err)
				return
			}

			contributionData[strconv.Itoa(year)] = daysForYear
			totalContributionsData[strconv.Itoa(year)] = total
		}(year)
	}

	// Data For Last Year
	wg.Add(1)
	go func() {
		defer wg.Done()
		total, lastYearData, err := cp.scrapeYearData(0, true)
		if err != nil {
			cp.logger.Error("Failed to scrape year data", "error", err)
			return
		}

		contributionData["last_year"] = lastYearData
		totalContributionsData["last_year"] = total
	}()

	wg.Wait()

	responseData := make(map[string]interface{})

	responseData["totals"] = totalContributionsData
	responseData["contributions"] = contributionData

	jsonData, err := json.Marshal(responseData)
	if err != nil {
		return []byte{}, errors.Wrap(err, "marshal json")
	}

	return jsonData, nil
}

func (cp *ContributionsParser) setStandardHeaders(req *http.Request) {
	req.Header.Set("referer", "https://github.com/"+cp.username)
	req.Header.Set("x-requested-with", "XMLHttpRequest")
}

// Github's contributions page has a link for each year you have made contributions.
func (cp *ContributionsParser) scrapeYears() ([]int, error) {
	url := "https://github.com/" + cp.username + "?action=show&controller=profiles&tab=contributions&user_id=" + cp.username
	// Get HTML.
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []int{}, errors.Wrap(err, "create request")
	}

	cp.setStandardHeaders(req)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []int{}, errors.Wrap(err, "send request")
	}

	defer resp.Body.Close()

	tokenizer := html.NewTokenizer(resp.Body)

	years := []int{}

	for {
		tt := tokenizer.Next()

		switch tt {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				return years, nil
			}

			return []int{}, errors.Wrap(tokenizer.Err(), "parse html")
		case html.StartTagToken:
			token := tokenizer.Token()
			if token.Data == "a" { // found anchor.
				for _, attr := range token.Attr {
					if attr.Key == "class" && strings.Contains(attr.Val, "js-year-link") {
						nextToken := tokenizer.Next()
						if nextToken == html.TextToken {
							yearText := strings.TrimSpace(tokenizer.Token().Data)
							if year, err := strconv.Atoi(yearText); err == nil {
								years = append(years, year)
							}
						}
					}
				}
			}
		}
	}
}

func (cp *ContributionsParser) scrapeYearData(year int, lastYear bool) (int, []Day, error) {
	cp.logger.Info("Scraping year data", "year", year, "lastYear", lastYear)

	url := "https://github.com/users/" + cp.username + "/contributions"
	if !lastYear {
		url += "?tab=overview&from=" + strconv.Itoa(year) + "-12-01&to=" + strconv.Itoa(year) + "-12-31"
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, []Day{}, errors.Wrap(err, "create request")
	}

	cp.setStandardHeaders(req)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, []Day{}, errors.Wrap(err, "send request")
	}

	defer resp.Body.Close()

	tokenizer := html.NewTokenizer(resp.Body)

	days := make(map[string]Day)
	total := 0

TokenizerLoop:
	for {
		tt := tokenizer.Next()

		switch tt {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				break TokenizerLoop
			}

			return 0, []Day{}, errors.Wrap(tokenizer.Err(), "parse html")
		case html.StartTagToken:
			token := tokenizer.Token()
			if token.Data == "td" {
				for _, attr := range token.Attr {
					if attr.Key == "data-date" {
						dayId, day, err := cp.parseDay(token)
						if err != nil {
							return 0, []Day{}, errors.Wrap(err, "parse day")
						}

						days[dayId] = day
					}
				}
			}

			if token.Data == "tool-tip" {
				dayIdTooltipIsFor := ""
				for _, attr := range token.Attr {
					if attr.Key == "for" {
						dayIdTooltipIsFor = attr.Val
					}
				}

				if day, ok := days[dayIdTooltipIsFor]; ok {
					next := tokenizer.Next()
					if next == html.TextToken {
						contributionsTxt := tokenizer.Token().Data

						var count int
						if strings.HasPrefix(contributionsTxt, "No contributions on") {
							count = 0
						} else {
							contributionsTooltipText := strings.Split(contributionsTxt, " ")
							count, err = strconv.Atoi(contributionsTooltipText[0])
							if err != nil {
								cp.logger.Error("Failed to parse count", "error", err, "text", contributionsTooltipText)
								return 0, []Day{}, errors.Wrap(err, "parse count")
							}
						}

						day.Count = count
						total += count
						days[dayIdTooltipIsFor] = day
					}
				}
			}
		}
	}

	allDays := []Day{}

	for _, day := range days {
		allDays = append(allDays, day)
	}

	// Sort days by date.
	slices.SortFunc(allDays, func(a, b Day) int {
		if a.Date.Before(b.Date) {
			return -1
		} else if a.Date.After(b.Date) {
			return 1
		}
		return 0
	})

	return total, allDays, nil
}

func (cp *ContributionsParser) parseDay(token html.Token) (string, Day, error) {
	dateRaw := ""
	level := -1
	dayId := ""
	for _, attr := range token.Attr {
		switch attr.Key {
		case "data-date":
			dateRaw = attr.Val
		case "data-level":
			levelParsed, err := strconv.Atoi(attr.Val)
			if err != nil {
				cp.logger.Error("Failed to parse level", "error", err)
				return "", Day{}, errors.Wrap(err, "parse level")
			}
			level = levelParsed
		case "id":
			dayId = attr.Val
		}
	}

	const layout = "2006-01-02" // Date format layout

	// Parse the date strings
	date, err := time.Parse(layout, dateRaw)
	if err != nil {
		return "", Day{}, errors.Wrap(err, "parse date")
	}

	return dayId, Day{
		Date:  date,
		Level: level,
	}, nil
}
