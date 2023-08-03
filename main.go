package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
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

func postHandler(il *ipLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !il.addIP(ip) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		var request postRequest
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&request)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Do something with request.ID
		fmt.Fprintf(w, "Received ID: %s\n", request.ID)
	}
}

func main() {
	il := &ipLimiter{ips: make(map[string]int), lastClean: time.Now()}
	http.HandleFunc("/post", postHandler(il))

	fmt.Println("Server is listening on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
