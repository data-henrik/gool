# This is how we want to name the binary output
BINARY=gool

# These are the values we want to pass for Version and BuildTime
VERSION=0.9.1

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS=-ldflags "-X github.com/mipimipi/gool/internal/release.Version=${VERSION} -X github.com/mipimipi/gool/internal/release.Build=`git rev-parse HEAD`"

all:
	go build ${LDFLAGS} -o ${GOPATH}/bin/${BINARY} cmd/main.go

$(GOMETALINTER):
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install &> /dev/null

.PHONY: lint
lint: $(GOMETALINTER)
	gometalinter ./... --vendor
