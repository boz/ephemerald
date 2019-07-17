GOCMD := GO111MODULE=auto go

.PHONY: build deps test test-nocache vet lint
build:
	$(GOCMD) build ./...

deps:
	glide install -v
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint

test:
	$(GOCMD) test ./...

test-nocache:
	$(GOCMD) test -count=1 ./...

vet:
	$(GOCMD) vet ./...

lint:
	golangci-lint run

.PHONY: server example clean
server:
	(cd cmd/ephemerald && $(GOCMD) build)

example:
	(cd _example && $(GOCMD) build -o example)

clean:
	rm _example/example cmd/ephemerald/ephemerald 2>/dev/null || true

.PHONY: devdeps
devdeps:
	go get github.com/golangci/golangci-lint/cmd/golangci-lint
