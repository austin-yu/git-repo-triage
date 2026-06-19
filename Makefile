.PHONY: build frontend clean dev

build: frontend
	mkdir -p bin
	go build -o bin/repo-triage

frontend:
	cd web && npm install && npm run build

clean:
	rm -rf bin web/dist

dev:
	go run main.go
