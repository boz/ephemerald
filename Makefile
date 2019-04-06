GOCMD := GO111MODULE=on go

build:
	$(GOCMD) build ./...

test:
	$(GOCMD) test ./...

vet:
	$(GOCMD) vet ./...

lint:
	govendor list +local | awk '{ print $$2 }' | xargs golint

server:
	(cd ephemerald && $(GOCMD) build)

example:
	(cd _example && $(GOCMD) build -o example)

install:
	$(GOCMD) install ./ephemerald

clean:
	rm _example/example ephemerald/ephemerald 2>/dev/null || true

release:
	GITHUB_TOKEN=$$GITHUB_REPO_TOKEN goreleaser

.PHONY: build test vet server example release clean
