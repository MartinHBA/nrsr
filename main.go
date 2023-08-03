package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
)

type postRequest struct {
	ID string `json:"id"`
}

type ipLimiter struct {
	mu        sync.RWMutex
	ips       map[string]int
	lastClean time.Time
}

func (il *ipLimiter) addIP(ip string) bool {
	il.mu.Lock()
	defer il.mu.Unlock()

	// Clean the IPs if 30 minutes have passed
	if time.Since(il.lastClean) > 30*time.Minute {
		il.ips = make(map[string]int)
		il.lastClean = time.Now()
	}

	// Check if the IP has reached the limit
	if il.ips[ip] >= 10 {
		return false
	}

	il.ips[ip]++
	return true
}

func getHandler(il *ipLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !il.addIP(ip) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		voteID := r.URL.Query().Get("id")
		if voteID == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		votes, err := getParliamentVotes(voteID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Error fetching votes: %s", err)
			return
		}

		jsonData, err := json.Marshal(votes)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Error encoding to JSON: %s", err)
			return
		}

		// Set content type to JSON and write the response
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(jsonData)
	}
}

// func main starts here
func main() {
	il := &ipLimiter{ips: make(map[string]int), lastClean: time.Now()}
	http.HandleFunc("/vote", getHandler(il))
	http.Handle("/docs/", http.StripPrefix("/docs", http.FileServer(http.Dir("./dist"))))
	fmt.Println("Server is listening on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getParliamentVotes(voteID string) (map[string][]string, error) {
	c := colly.NewCollector()

	votes := make(map[string][]string)

	var voteType string

	c.OnHTML(".hpo_result_table", func(e *colly.HTMLElement) {
		voteType = ""
		e.DOM.Find("tr").Each(func(i int, s *goquery.Selection) { // Loop over <tr> elements
			voteTypeCell := s.Find(".hpo_result_block_title")
			if voteTypeCell.Length() > 0 {
				voteType = strings.TrimSpace(voteTypeCell.Text())
				return // Skip the rest of this loop iteration if it's a vote type row
			}

			s.Find("td").Each(func(j int, td *goquery.Selection) {
				name := strings.TrimSpace(td.Text())
				if name != "" && voteType != "" {
					votes[voteType] = append(votes[voteType], name)
				}
			})
		})
	})

	url := fmt.Sprintf("https://www.nrsr.sk/web/Default.aspx?sid=schodze/hlasovanie/hlasovanie&ID=%s", voteID)
	err := c.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("could not visit page: %s", err)
	}

	fmt.Println(votes)
	return votes, nil
}

/*
$uri = "https://nrsr.azurewebsites.net/vote"
#$uri = "http://localhost:8080/vote"
$body = @"
{
    "id" : "48180"
}
"@

$result = Invoke-WebRequest -Uri $uri -Method Post -Body $body -ContentType 'application/json' -UseBasicParsing
$table = $result.Content | ConvertFrom-Json
$table | ft *
*/
