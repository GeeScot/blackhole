#!/bin/sh
cd "$(dirname "$0")"

git pull
go build

./blackhole -target=sources/default.json
./blackhole -target=sources/strict.json

now=$(date)

git add .
git commit -m "Auto Update: $now"
git push
