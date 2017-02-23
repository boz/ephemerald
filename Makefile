
build:
	govendor build +local

test:
	govendor test +local -race

vet:
	govendor vet +local
