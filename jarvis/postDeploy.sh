#!/usr/bin/env bash

make bin=braft jarvis
mkdir -p build
cp braft ./build && cp jarvis/* ./build
mkdir -p output
cd build && tar -cvzf ../output/ROOT.tar.gz *
