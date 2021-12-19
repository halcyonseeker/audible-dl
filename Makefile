
default:
	go build

install:
	mkdir -p /usr/local/bin/
	cp audible-dl /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/audible-dl

clean:
	rm -f audible-dl
