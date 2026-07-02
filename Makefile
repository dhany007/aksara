IMAGE = adipatidhany/aksara:latest
VENV_DIR ?= .venv
PYTHON_BIN ?= $(VENV_DIR)/bin/python

# Install Python dependencies for local PDF extraction.
python-deps: $(PYTHON_BIN)
	$(PYTHON_BIN) -m pip install -r requirements.txt

$(PYTHON_BIN):
	python3 -m venv $(VENV_DIR)

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

.PHONY: python-deps translate docker-translate build test push
