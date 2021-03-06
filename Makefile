build:
	govendor build +local

test:
	govendor test +local -race

vet:
	govendor vet +local

lint:
	govendor list +local | awk '{ print $$2 }' | xargs golint

server:
	(cd ephemerald && go build)

example:
	(cd _example && go build -o example)

install:
	go install ./ephemerald

clean:
	rm _example/example ephemerald/ephemerald 2>/dev/null || true

release:
	GITHUB_TOKEN=$$GITHUB_REPO_TOKEN goreleaser

.PHONY: build test vet server example release clean
