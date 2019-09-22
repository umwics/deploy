#!/usr/bin/env sh

GOOS=linux go build -o bin/deploy
./layer/build.sh no
