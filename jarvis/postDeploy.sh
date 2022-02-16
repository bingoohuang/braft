#!/usr/bin/env bash

export GOPROXY=https://goproxy.cn
make bin=braft jarvis

mkdir -p build
cp braft ./build && cp jarvis/* ./build && rm -f ./build/postDeploy.sh
mkdir -p output
cd build && tar -cvzf ../output/ROOT.tar.gz * && tar -ztvf ../output/ROOT.tar.gz
