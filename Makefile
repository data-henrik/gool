# This is how we want to name the binary output
BINARY=gool

# These are the values we want to pass for Version and BuildTime
VERSION=0.9.2

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS=-ldflags "-X github.com/mipimipi/gool/src.version=${VERSION}"

all:
	cd src
	go build ${LDFLAGS} -o ../${BINARY}

$(GOMETALINTER):
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install &> /dev/null

.PHONY: lint
lint: $(GOMETALINTER)
	gometalinter ./... --vendor
