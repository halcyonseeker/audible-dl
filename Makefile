
all: audible-dl

install: audible-dl
	mkdir -p /usr/local/bin/
	cp audible-dl /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/audible-dl

audible-dl:
	go build

clean:
	rm -f audible-dl
