#!/usr/bin/env bash
version=local-$(git rev-parse HEAD)
time=$(date)
echo "$version"
go test .
go build -o .build/k8ConsoleViewer \
         -ldflags="-X 'github.com/JLevconoks/k8ConsoleViewer/version.BuildTime=$time' -X 'github.com/JLevconoks/k8ConsoleViewer/version.BuildVersion=$version'" .