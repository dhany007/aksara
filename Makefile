IMAGE = adipatidhany/aksara:latest

# Translate local books with Go.
translate:
	go run ./cmd/aksara

# Translate local books inside Docker.
docker-translate:
	docker compose run --rm app

# Build the Docker image.
build:
	docker compose build

# Run Go tests.
test:
	go test ./...

# Build and push the image for linux/amd64.
push:
	docker buildx build --platform linux/amd64 -t $(IMAGE) --push .

.PHONY: translate docker-translate build test push
