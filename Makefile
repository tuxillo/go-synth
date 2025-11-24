# Makefile for dsynth-go

PROG=		dsynth
SRCS=		$(shell find . -name '*.go' -not -path './vendor/*')
VERSION=	2.0.0

PREFIX?=	/usr/local
BINDIR=		$(PREFIX)/bin
MANDIR=		$(PREFIX)/man/man1

GO?=		go
GOFLAGS=	-ldflags "-X main.Version=$(VERSION)"

all: build

build: $(PROG)

$(PROG): $(SRCS)
	$(GO) build $(GOFLAGS) -o $(PROG) .

install: $(PROG)
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 $(PROG) $(DESTDIR)$(BINDIR)/

clean:
	rm -f $(PROG)
	$(GO) clean

test:
	$(GO) test -v ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

.PHONY: all build install clean test fmt vet