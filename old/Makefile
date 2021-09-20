all: audible-dl

audible-dl: audible-dl.go config.go scraper.go downloader.go converter.go
	go build $^

install: audible-dl
	cp audible-dl /usr/local/bin

uninstall:
	rm /usr/local/bin/audible-dl

clean:
	rm -f audible-dl
