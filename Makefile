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
	(cd example && go build)

.PHONY: build test vet server example
