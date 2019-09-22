package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/mholt/archiver"
)

const (
	// RepoOwner is the owner of the repository.
	RepoOwner = "umwics"
	// RepoName is the name of the repository.
	RepoName = "wics-site"
	// RemotePath is the path to the site on the remote server.
	RemotePath = "wics@aviary.cs.umanitoba.ca:~/public_html"
	// SSHKey is the path to our SSH key.
	SSHKey = "ssh/wics"
)

var (
	// RepoURL is the download URL of the repo.
	RepoURL = fmt.Sprintf("https://github.com/%s/%s/archive/master.zip", RepoOwner, RepoName)
	// WebhookSecret is the secret for GitHub.
	WebhookSecret = []byte(os.Getenv("WEBHOOK_SECRET"))
)

type (
	// Request is the type we get from Lambda.
	Request events.APIGatewayProxyRequest
	// Response is the type we give back to Lambda.
	Response events.APIGatewayProxyResponse
)

// Validate validates the request.
func (req Request) Validate(resp *Response) bool {
	return true // TODO
}

// downloadRepo downloads and unzip the repository, and returns its directory.
func downloadRepo() (string, error) {
	resp, err := http.Get(RepoURL)
	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	f, err := ioutil.TempFile("", "wics.*.zip")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())

	if _, err = f.Write(b); err != nil {
		return "", err
	}

	dir, err := ioutil.TempDir("", "wics")
	if err != nil {
		return "", err
	}

	if err = archiver.Unarchive(f.Name(), dir); err != nil {
		return "", err
	}

	return filepath.Join(dir, fmt.Sprintf("%s-master", RepoName)), nil
}

// buildSite builds the site with Jekyll.
func buildSite(dir string) (string, error) {
	if err := doCmd(dir, "jekyll", "build"); err != nil {
		return "", err
	}
	return filepath.Join(dir, "_site"), nil
}

// syncSite pushes the site to the remote server with rsync.
func syncSite(dir string) error {
	defer os.RemoveAll(dir)

	// To recursively sync the directory, the path must end with a slash.
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	// We need to use a custom SSH command to use our bundled SSH key.
	sshCmd := fmt.Sprintf("ssh -i %s", SSHKey)

	return doCmd("rsync", "-e", sshCmd, "-a", "--delete", dir, RemotePath)
}

// doCmd runs some shell command and prints its output.
func doCmd(dir, program string, args ...string) error {
	cmd := exec.Command(program, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	b, err := cmd.CombinedOutput()
	if len(b) > 0 {
		fmt.Println(string(b))
	}
	return err
}

func main() {
	lambda.Start(func(req Request) (resp Response, nillErr error) {
		if req.Validate(&resp) {
			resp.StatusCode = 500

			dir, err := downloadRepo()
			if err != nil {
				resp.Body = fmt.Errorf("download repo: %w", err).Error()
				return
			}

			dir, err = buildSite(dir)
			if err != nil {
				resp.Body = fmt.Errorf("build site: %w", err).Error()
				return
			}

			if err = syncSite(dir); err != nil {
				resp.Body = fmt.Errorf("sync site: %w", err).Error()
				return
			}

			resp.StatusCode = 200
		}

		return
	})
}
