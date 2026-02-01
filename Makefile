.PHONY: build run clean

build:
	@echo building...
	@go build -gcflags="-m" -o ./build/dman ./cmd/main

run:
	@go run ./cmd/main

clean:
	@echo cleaning...
	@rm -rf ./build
