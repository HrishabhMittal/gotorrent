.PHONY: build run clean

build:
	@echo building...
	@go build -o ./build/gotorrent ./cmd/gotorrent

run:
	@go run ./cmd/main

clean:
	@echo cleaning...
	@rm -rf ./build
