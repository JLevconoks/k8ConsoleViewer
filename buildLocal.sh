#!/usr/bin/env bash
version=local-$(git describe --tags)
time=$(date)
echo "$version"
go test ./...
go build -o .build/k8ConsoleViewer \
         -ldflags="-X 'github.com/JLevconoks/k8ConsoleViewer/cmd.buildTime=$time' -X 'github.com/JLevconoks/k8ConsoleViewer/cmd.buildVersion=$version'" .