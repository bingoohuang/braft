#!/usr/bin/env bash

export GOPROXY=https://goproxy.cn
make bin=braft jarvis

tar -cvzf ROOT.tar.gz braft

mkdir -p output

mv ROOT.tar.gz output/
mv jarvis/* output/
