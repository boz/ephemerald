GOCMD := GO111MODULE=auto go

build:
	$(GOCMD) build ./...

deps:
	glide install -v

test:
	$(GOCMD) test ./...

vet:
	$(GOCMD) vet ./...

server:
	(cd cmd/ephemerald && $(GOCMD) build)

example:
	(cd _example && $(GOCMD) build -o example)

install:
	$(GOCMD) install ./ephemerald

clean:
	rm _example/example cmd/ephemerald/ephemerald 2>/dev/null || true

release:
	GITHUB_TOKEN=$$GITHUB_REPO_TOKEN goreleaser

.PHONY: build deps test vet server example release clean
