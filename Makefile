.DEFAULT_GOAL = full_build

PACKAGES = $(shell go list ./... | grep -v /vendor/)

build:
	@echo Building sources
	@go build $(PACKAGES)

test:
	@echo Running tests
	@echo 'mode: atomic' > coverage.txt && go list ./... | xargs -n1 -I{} sh -c 'go test -covermode=atomic -coverpkg ./... -coverprofile=coverage.tmp {} && tail -n +2 coverage.tmp >> coverage.txt' && rm coverage.tmp 
	@go tool cover -html=coverage.txt -o=coverage.html

benchmark:
	@echo Benchmarking
	@go test -bench .
	@cd ./stores/in-memory; go test -bench .; cd ../..
	@cd ./stores/mongo; go test -bench .; cd ../..

examples:
	@echo Building examples
	@go build ./examples/....

full_build: build test benchmark examples
