package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

var pages *Pages

func main() {
	repositories := os.Getenv("GITEA_PAGES_REPOSITORIES")
	if repositories == "" {
		log.Fatal("GITEA_PAGES_REPOSITORIES is unset")
	}

	target := os.Getenv("GITEA_PAGES_TARGET")
	if target == "" {
		log.Fatal("GITEA_PAGES_TARGET is unset")
	}

	listenAddr := os.Getenv("GITEA_PAGES_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":3000"
	}

	token := os.Getenv("GITEA_PAGES_TOKEN")
	if repositories == "" {
		log.Fatal("GITEA_PAGES_TOKEN is unset")
	}

	pages = NewPages(repositories, target, token)

	go periodicSync()

	webhookReceiver(listenAddr)
}

func periodicSync() {
	for {
		pages.fullSync()
	}
}

func webhookReceiver(listenAddr string) {
	// Run web server to receive push webhooks
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", handleWebhook)
	server := http.Server{
		Addr:	 listenAddr,
		Handler: mux,
	}
	log.Printf("Listening on %s", listenAddr)
	err := server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func sendError(res http.ResponseWriter, message string) {
	res.WriteHeader(400)
	io.WriteString(res, message)
}

type Repository struct {
	FullName string `json:"full_name"`
}

type Event struct {
	Repository Repository `json:"repository"`
}

func handleWebhook(res http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		sendError(res, "invalid method")
		return
	}

	// Check token
	token := req.Header["Authorization"]
	if len(token) != 1 || token[0] != "Bearer " + pages.token {
		res.WriteHeader(410)
		io.WriteString(res, "invalid token")
		return
	}

	var event Event
	content, err := io.ReadAll(req.Body)
	if err != nil {
		sendError(res, "error reading request body")
		return
	}
	err = json.Unmarshal(content, &event)
	if err != nil {
		sendError(res, "error parsing request body")
		return
	}

	pages.syncRepo(event.Repository.FullName)
}

type Pages struct {
	repositories string
	target		 string
	token		 string
	mutex		 sync.Mutex
}

func NewPages(repositories string, target string, token string) *Pages {
	return &Pages{
		repositories: repositories,
		target:		  target,
		token:		  token,
	}
}

func (p *Pages) fullSync() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// TODO: Check all repositories
}

func (p *Pages) syncRepo(repo string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// TODO: Update one repository
}
