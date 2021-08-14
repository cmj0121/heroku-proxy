BIN := bin/heroku-proxy
SRC := $(wildcard *.go)

.PHONY: all clean help lint get

all: $(BIN) # build all binary
	 @pre-commit install --install-hooks

clean:		# clean-up the environment
	rm -f $(BIN)

help:		# show this message
	@printf "Usage: make [OPTION]\n"
	@printf "\n"
	@perl -nle 'print $$& if m{^[\w-]+:.*?#.*$$}' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?#"} {printf "    %-18s %s\n", $$1, $$2}'

get:		# get the necessary library
	go get -d ./...

$(BIN): $(SRC)
	go build -ldflags="-s -w" -o $@ $(SRC)

lint: $(SRC)
	gofmt -w -s $(SRC)
