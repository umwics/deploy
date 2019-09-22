// Webhook handler for GitHub push events.
// Synchronizes the live website with the current master branch of umwics/wics-site.
//
// Required environment variables:
// - REPO_PATH
// - WEBHOOK_SECRET
// - WICS_KEY
//
// System dependencies:
// - bundler
// - git
// - rsync
// - zlib (zlib1g-dev on Ubuntu)
//
// Other requirements:
// - The remote server must have authorized the client's public SSH key.
// - This must run on Linux (or at least a system with '/'-joined paths).
// - The client must have the repository cloned to $REPO_PATH.
//
// TODO: Check the remote server for the new files after they've been copied.
// TODO: Maybe use github.com/rjz/githubhook and github.com/google/go-github.

package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
)

const (
	// Port listened on by the server.
	port = 4000
	// HTTP endpoint to trigger a sync.
	endpoint = "/sync"
	// FTP server address and destination.
	serverStr = "wics@aviary.cs.umanitoba.ca:~/public_html"
)

var (
	// Path to the local repository.
	repoPath = os.Getenv("REPO_PATH")
	// Allows us to verify that requests are coming from GitHub.
	webhookSecret = os.Getenv("WEBHOOK_SECRET")
	// Key for manually trigger synchronization.
	wicsKey = os.Getenv("WICS_KEY")
)

func init() {
	if repoPath == "" {
		log.Fatal("environment variable REPO_PATH is not set")
	}
	if webhookSecret == "" {
		log.Fatal("environment variable WEBHOOK_SECRET is not set")
	}
	if wicsKey == "" {
		log.Fatal("environment variable WICS_KEY is not set")
	}
}

func main() {
	log.Println("starting server")
	http.HandleFunc(endpoint, handler)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

// handler handles the push event.
// See: https://developer.github.com/v3/activity/events/types/#pushevent
func handler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if err := filter(w, r); err != nil {
		log.Println("request did not meet criteria:", err)
		return
	}

	if err := syncSite(); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// syncSite synchronizes the live website with the repository's master branch.
func syncSite() error {
	log.Println("synchronizing")

	log.Println("updating local repository")
	cmd := exec.Command("git", "-C", repoPath, "fetch")
	if err := withOutput(cmd); err != nil {
		return errors.Wrap(err, "git fetch")
	}

	log.Println("resetting to latest master")
	cmd = exec.Command("git", "-C", repoPath, "reset", "--hard", "origin/master")
	if err := withOutput(cmd); err != nil {
		return errors.Wrap(err, "git reset")
	}

	log.Println("installing site dependencies")
	cmd = exec.Command("bundle", "install", "--path", "vendor/bundle")
	cmd.Dir = repoPath
	if err := withOutput(cmd); err != nil {
		return errors.Wrap(err, "bundle install")
	}

	log.Println("building site")
	cmd = exec.Command("bundle", "exec", "jekyll", "build")
	cmd.Dir = repoPath
	if err := withOutput(cmd); err != nil {
		return errors.Wrap(err, "jekyll build")
	}

	log.Println("synchronizing built site with remote server")
	cmd = exec.Command("rsync", "-a", "--delete", repoPath+"/_site/", serverStr)
	if err := withOutput(cmd); err != nil {
		return errors.Wrap(err, "rsync")
	}

	log.Println("success!")
	return nil
}

// filter checks that the request meets the criteria for synchronization.
func filter(w http.ResponseWriter, r *http.Request) error {
	// WICS key gets priority.
	if wk := r.Header.Get("X-WICS-Key"); wk == wicsKey {
		return nil
	} else if wk != "" {
		w.WriteHeader(http.StatusForbidden)
		return errors.New("incorrect WICS key")
	}

	// If WICS key is missing, check for GitHub signature.
	hs := r.Header.Get("X-Hub-Signature")
	if hs == "" {
		w.WriteHeader(http.StatusForbidden)
		return errors.New("missing GitHub signature")
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return err
	}
	if !checkSig(b, hs) {
		w.WriteHeader(http.StatusForbidden)
		return errors.New("incorrect signature")
	}

	// Check that the push is on master branch.
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return err
	}
	if ref, ok := m["ref"]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		return errors.New("missing key 'ref'")
	} else if ref != "refs/heads/master" {
		w.WriteHeader(http.StatusOK)
		return errors.Errorf("not master branch: %s", ref)
	}

	return nil
}

// checkSig verifies that a request is coming from GitHub.
// See: https://developer.github.com/webhooks/securing
// Implementation from github.com/rjz/githubhook.
func checkSig(body []byte, sig string) bool {
	h := hmac.New(sha1.New, []byte(webhookSecret))
	h.Write(body)
	b := make([]byte, 20)
	hex.Decode(b, []byte(sig[5:]))
	return hmac.Equal(h.Sum(nil), b)
}

// withOutput runs a command and prints its output.
func withOutput(cmd *exec.Cmd) error {
	b, err := cmd.CombinedOutput()
	fmt.Print(string(b))
	return err
}
