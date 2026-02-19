frontend-build:
	cd frontend && npm install && npm run build

build: frontend-build
	go build -v -trimpath -o the-only-mirror ./
