.PHONY: build frontend clean dev

build: frontend
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -o bin/repo-triage-windows-amd64.exe

frontend:
	cd web && npm install && npm run build

clean:
	rm -rf bin web/dist

dev:
	go run main.go
