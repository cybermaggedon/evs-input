
# Create version tag from git tag
VERSION=$(shell git describe | sed 's/^v//')
REPO=cybermaggedon/evs-input
DOCKER=docker
GO=GOPATH=$$(pwd)/go go

all: evs-input build

evs-input: input.go go.mod go.sum
	${GO} build -o $@ input.go

build: evs-input
	${DOCKER} build -t ${REPO}:${VERSION} -f Dockerfile .
	${DOCKER} tag ${REPO}:${VERSION} ${REPO}:latest

push:
	${DOCKER} push ${REPO}:${VERSION}
	${DOCKER} push ${REPO}:latest


