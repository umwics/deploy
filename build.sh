#!/usr/bin/env sh

GOOS=linux go build -ldflags="-s -w" -o bin/deploy
ln -sf deploy bin/webhook

zip -rq ssh.zip ssh

./layer/build.sh no
