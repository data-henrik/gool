# This is how we want to name the binary output
BINARY=gool

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS=-ldflags "-X github.com/mipimipi/gool/main.Version=${VERSION}"

all:
	go build ${LDFLAGS} -o ${BINARY}

$(GOMETALINTER):
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install &> /dev/null

.PHONY: lint
lint: $(GOMETALINTER)
	gometalinter ./... --vendor

install:
	install -Dm755 ${BINARY} $(DESTDIR)/usr/bin/${BINARY}
	install -Dm644 resources/gool.desktop $(DESTDIR)/usr/share/applications/gool.desktop
	install -Dm644 resources/otrkey_mime.xml $(DESTDIR)/usr/share/mime/packages/otrkey_mime.xml