frontend-build:
	cd frontend && npm ci && npm run build

build: frontend-build
	go build -v -trimpath -o the-only-mirror ./
