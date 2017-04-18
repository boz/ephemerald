build:
	govendor build +local

test:
	govendor test +local -race

vet:
	govendor vet +local

server:
	(cd ephemerald && go build)

example:
	(cd example && go build)

.PHONY: build test vet server example
