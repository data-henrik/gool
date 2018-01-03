# This is how we want to name the binary output
BINARY=gool

# These are the values we want to pass for Version and BuildTime
VERSION=0.9.5

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS=-ldflags "-X github.com/mipimipi/gool/main.Version=${VERSION} -X github.com/mipimipi/main.Build=`git rev-parse HEAD`"

all:
	go build ${LDFLAGS} -o ${BINARY}

$(GOMETALINTER):
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install &> /dev/null

.PHONY: lint
lint: $(GOMETALINTER)
	gometalinter ./... --vendor

install:
	# copy binary to global bin directory
	cp	${BINARY} /usr/bin/${BINARY}
	# copy desktop file to global directory
	cp resources/gool.desktop /usr/share/applications
	# install mime type for ".otrkey files"
	xdg-mime install --novendor resources/otrkey_mime.xml 
	# set gool as default application
	xdg-mime default gool.desktop application/x-onlinetvrecorder