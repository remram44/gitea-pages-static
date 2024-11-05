package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const FULL_SYNC_INTERVAL = 5 * time.Minute

const BRANCH_NAME = "gitea-pages"

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
	if token == "" {
		log.Fatal("GITEA_PAGES_TOKEN is unset")
	}

	pages = NewPages(repositories, target, token)

	go periodicSync()

	webhookReceiver(listenAddr)
}

func periodicSync() {
	for {
		if err := pages.fullSync(); err != nil {
			log.Printf("Error: %v", err)
		}
		time.Sleep(FULL_SYNC_INTERVAL)
	}
}

func webhookReceiver(listenAddr string) {
	// Run web server to receive push webhooks
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", handleWebhook)
	server := http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	log.Printf("Listening on %s", listenAddr)
	err := server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

type Repository struct {
	FullName string `json:"full_name"`
}

type Event struct {
	Repository Repository `json:"repository"`
}

func handleWebhook(res http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(res, "invalid method", 400)
		return
	}

	// Check token
	token := req.Header["Authorization"]
	if len(token) != 1 || token[0] != "Bearer "+pages.token {
		http.Error(res, "invalid token", 410)
		return
	}

	// Parse event JSON
	var event Event
	content, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(res, "error reading request body", 400)
		return
	}
	err = json.Unmarshal(content, &event)
	if err != nil {
		http.Error(res, "error parsing request body", 400)
		return
	}

	// Update single repository
	log.Printf("Got webhook for %s", event.Repository.FullName)
	pages.syncRepo(event.Repository.FullName)

	res.WriteHeader(204)
}

func twoLevelDirs(root string, f func(string)) error {
	firstLevelDirs, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, dir1 := range firstLevelDirs {
		if !dir1.IsDir() {
			continue
		}
		dir1s := filepath.Join(root, dir1.Name())
		secondLevelDirs, err := os.ReadDir(dir1s)
		if err != nil {
			return err
		}

		for _, dir2 := range secondLevelDirs {
			if !dir2.IsDir() {
				continue
			}
			f(dir1.Name() + "/" + dir2.Name())
		}
	}

	return nil
}

type Pages struct {
	repositories string
	target       string
	token        string
	mutex        sync.Mutex
}

func NewPages(repositories string, target string, token string) *Pages {
	return &Pages{
		repositories: repositories,
		target:       target,
		token:        token,
	}
}

func (p *Pages) fullSync() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	log.Print("Doing full sync")

	// Build list of available repositories
	available := make(map[string]struct{})
	err := twoLevelDirs(p.repositories, func(name string) {
		if strings.HasSuffix(name, ".git") {
			available[name[:len(name)-4]] = struct{}{}
		}
	})
	if err != nil {
		return err
	}

	// Build list of deployed repositories
	deployed := make(map[string]struct{})
	err = twoLevelDirs(p.target, func(name string) {
		deployed[name] = struct{}{}
	})
	if err != nil {
		return err
	}

	// Remove deployed repositories that no longer exist
	for repo := range deployed {
		gitDir := p.getGitDir(repo)

		_, exists := available[repo]
		if !exists {
			log.Printf("full sync: Removing deployment, repo is gone: %s", repo)
			p.removeRepo(repo)
			continue
		}

		cmd := exec.Command("git", "rev-parse", BRANCH_NAME)
		cmd.Env = append(cmd.Env, "GIT_DIR="+gitDir)
		err := cmd.Run()
		if err != nil {
			log.Printf("full sync: Removing deployment, repo's %s branch is gone: %s", BRANCH_NAME, repo)
			p.removeRepo(repo)
			continue
		}

		log.Printf("full sync: ok: %s", repo)
	}

	// Update or create repositories
	for repo := range available {
		gitDir := p.getGitDir(repo)

		cmd := exec.Command("git", "rev-parse", BRANCH_NAME)
		cmd.Env = append(cmd.Env, "GIT_DIR="+gitDir)
		err := cmd.Run()
		if err != nil {
			continue
		}

		p.writeRepo(repo)
	}

	return nil
}

func (p *Pages) syncRepo(repo string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	gitDir := p.getGitDir(repo)

	if _, err := os.Stat(gitDir); err == nil {
		p.writeRepo(repo)
	} else {
		p.removeRepo(repo)
	}
}

func (p *Pages) getGitDir(repo string) string {
	return filepath.Join(p.repositories, repo+".git")
}

func (p *Pages) getDeployDir(repo string) string {
	return filepath.Join(p.target, repo)
}

func (p *Pages) writeRepo(repo string) {
	gitDir := p.getGitDir(repo)
	deployDir := p.getDeployDir(repo)

	if err := os.MkdirAll(deployDir, 0755); err != nil {
		log.Printf("Error updating site: %v", err)
	}
	cmd := exec.Command("git", "restore", "-s", BRANCH_NAME, "--worktree", "--no-overlay", ".")
	cmd.Dir = deployDir
	cmd.Env = append(cmd.Env, "GIT_WORK_TREE="+deployDir)
	cmd.Env = append(cmd.Env, "GIT_DIR="+gitDir)
	if err := cmd.Run(); err != nil {
		log.Printf("Error updating site: %v", err)
	}
}

func (p *Pages) removeRepo(repo string) {
	deployDir := p.getDeployDir(repo)

	os.RemoveAll(deployDir)
}
