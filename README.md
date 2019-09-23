# WICS Site Auto-Deployment

This service deploys new changes to the website repo to the live site!

### Setup

You must have these things installed:

- [Docker](https://docker.com)
- [Go compiler](https://golang.org)
- [Serverless Framework](https://serverless.com/framework)

You will also need an [AWS](https://aws.amazon.com) account with credentials set up on your machine, and shell access to the remote server (`wics@aviary.cs.umanitoba.ca`in this case).
The server must have `rsync` installed.

Deploy the service like so (these steps assume Linux or MacOS):

- Generate an SSH key: `mkdir -p ssh; yes | ssh-keygen -f ssh/wics`.
  Then, log into `wics@aviary.cs.umanitoba.ca` (using your own key or password) and add the contents of `ssh/wics.pub` to `~/.ssh/authorized_keys`.
- Generate a secure secret.
  You can do this with something like `head -c256 /dev/urandom | md5sum | cut -d' ' -f1` (just `md5` on MacOS) or you can mash your keyboard.
  Save this secret somewhere.
  Then, run `export WEBHOOK_SECRET=<the-secret>`
- Run `build.sh`.
  This will take quite a while, especially the first time.
- Run `serverless deploy --stage prod`.
  This will also take a while the first time.
  At the end, a URL should appear that ends in `/prod/deploy`.
- Go to the [create webhook page](https://github.com/umwics/wics-site/settings/hooks/new) to create a new webhook.
  Set the payload URL to the URL you saw in the last step.
  Set the secret to your `WEBHOOK_SECRET`.
  Press "Add webhook", and you're done!

You can test the synchronization step by running `serverless invoke --function deploy --log`.
If everything is set up properly, you should see the message "Successfully synchronized site".
