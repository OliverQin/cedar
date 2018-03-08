default: fmt vet test

# Taken from https://github.com/Shopify/sarama/blob/master/Makefile
# Taken from https://github.com/codecov/example-go#caveat-multiple-files

test:
	echo "" > coverage.txt
	for d in `go list ./... | grep -v vendor`; do \
		go test -p 1 -v -timeout 90s -race -coverprofile=profile.out -covermode=atomic $$d || exit 1; \
		if [ -f profile.out ]; then \
			cat profile.out >> coverage.txt; \
			rm profile.out; \
		fi \
	done

vet:
	go vet ./...

fmt:
	@if [ -n "$$(go fmt ./...)" ]; then echo 'Please run go fmt on your code.' && exit 1; fi

