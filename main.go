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
	runtime "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/google/go-github/v28/github"
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
	// DeployFunction is the name of the deploy Lambda function.
	DeployFunction = os.Getenv("DEPLOY_FUNCTION")
	// Lambda is an AWS Lambda client.
	Lambda = lambda.New(session.Must(session.NewSession()))
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

// AsHTTP returns a http.Request with its Body set to that of req.
func (req Request) AsHTTP() *http.Request {
	return &http.Request{Body: ioutil.NopCloser(strings.NewReader(req.Body))}
}

// Validate validates the request.
func (req Request) Validate() error {
	r := req.AsHTTP()

	payload, err := github.ValidatePayload(r, WebhookSecret)
	if err != nil {
		return err
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return err
	}

	switch event.(type) {
	case github.PushEvent:
		pe := event.(github.PushEvent)
		// The branch should be the default branch.
		expected := fmt.Sprintf("refs/heads/%s", pe.GetRepo().GetDefaultBranch())
		if pe.GetRef() != expected {
			return fmt.Errorf("Ref %s is not the default branch", pe.GetRef())
		}
	default:
		// The event should be a push event.
		return fmt.Errorf("Unknown event type %T", event)
	}

	return nil
}

// downloadRepo downloads and unzip the repository, and returns its directory.
func downloadRepo() (string, error) {
	resp, err := http.Get(RepoURL)
	if err != nil {
		return "", err
	} else if resp.StatusCode != 200 {
		return "", fmt.Errorf("Bad status code: %d", resp.StatusCode)
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

	return doCmd("", "rsync", "-e", sshCmd, "-a", "--delete", dir, RemotePath)
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
	runtime.Start(func(req Request) (resp Response, err error) {
		if strings.HasSuffix(os.Args[0], "webhook") {
			resp.StatusCode = 200

			// Make sure that the event is a push to the master branch.
			if err = req.Validate(); err != nil {
				fmt.Println("validate request:", err)
				resp.StatusCode = 400
			}

			// Invoke the deploy function so that it can have more time to run and the chance to retry.
			input := lambda.InvokeInput{
				FunctionName:   aws.String(DeployFunction),
				InvocationType: aws.String("Event"),
			}
			if _, err = Lambda.Invoke(&input); err != nil {
				fmt.Println("invoke function:", err)
				resp.StatusCode = 500
			}

			return resp, nil
		} else if strings.HasSuffix(os.Args[0], "deploy") {
			var dir string

			// Download the site repository.
			if dir, err = downloadRepo(); err != nil {
				fmt.Println("download repo:", err)
				return
			}

			// Build the site.
			if dir, err = buildSite(dir); err != nil {
				fmt.Println("build site:", err)
				return
			}

			// Sync the site with the remote server.
			if err = syncSite(dir); err != nil {
				fmt.Println("sync site:", err)
				return
			}

			return resp, nil
		} else {
			fmt.Println("unknown command:", os.Args[0])
			resp.StatusCode = 400
			return resp, nil
		}
	})
}
