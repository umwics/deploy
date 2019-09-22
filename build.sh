#!/usr/bin/env sh

GOOS=linux go build -ldflags="-s -w" -o bin/deploy
ln -sf deploy bin/webhook

./layer/build.sh no
