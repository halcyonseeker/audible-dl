
default:
	go build

install:
	mkdir -p /usr/local/bin/
	mkdir -p /usr/local/share/man/man1/
	cp audible-dl /usr/local/bin/
	cp audible-dl.1 /usr/local/share/man/man1

uninstall:
	rm -f /usr/local/bin/audible-dl
	rm -f /usr/local/share/man/man1/audible-dl.1

doc:
	echo '```' >audible-dl.1.md
	MANWIDTH=80 man ./audible-dl.1 >>audible-dl.1.md
	echo '```' >>audible-dl.1.md

clean:
	rm -f audible-dl
