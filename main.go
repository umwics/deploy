package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// SSHKey is the path to our SSH key.
const SSHKey = "ssh/wics"

// WebhookSecret is the secret for GitHub.
var WebhookSecret = []byte(os.Getenv("WEBHOOK_SECRET"))

type (
	// Request is the type we get from Lambda.
	Request events.APIGatewayProxyRequest
	// Response is the type we give back to Lambda.
	Response events.APIGatewayProxyResponse
)

func main() {
	lambda.Start(func(req Request) (resp Response, nillErr error) {
		if b, err := exec.Command("rsync", "--version").CombinedOutput(); err != nil {
			log.Println("rsync:", err)
		} else {
			fmt.Println(string(b))
		}
		if b, err := exec.Command("jekyll", "-v").CombinedOutput(); err != nil {
			log.Println("jekyll:", err)
		} else {
			fmt.Println(string(b))
		}
		resp.StatusCode = 200
		return
	})
}
