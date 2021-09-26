audible-dl --- An archiver for your Audible library
===================================================

**This program is a work in progress.  At present it doesn't do
concurrent downloads, so if you have a large library it might take
several days to download everything.**

Audible-dl is an archiving tool that keeps an up-to-date, offline,
archive of the audiobooks you've purchased with Audible as DRM-free,
metadata rich, opus files.

It is intended for individuals who wish to effortlessly maintain an
offline store of their Audiobooks to browse and listen to on their own
terms.  Please do not use audible-dl to upload books to piracy sites;
authors and narrators need to make a living somehow.

Install
-------
Audible-dl depends on ffmpeg and the Go programming language. You
should be able to build it for any OS supported by the Go compiler,
however I've only tested it on Arch GNU/Linux and FreeBSD.  Build it
with `go build`.

User Guide
----------
Audible-dl stores all your books in the current working directory.  To
set it up, you need three things.

1. A directory to store your audiobooks in (eg, `~/Audiobooks`),

2. Your activation bytes.  There are a number of guides online of how
   to get them.

3. An archive of your Audible login cookies.  Open a browser, log into
   Audible, navigate to <https://audible.com/library/titles>, open the
   network tab in inspect element, and reload the page.  Right click
   on the `/library/titles` GET request and select `Save as HAR`.

Now navigate to your audiobooks directory and run

    audible-dl -i [path/to/request.har] -b [bytes]

This will cache your authentication cookies and activation bytes in
the file `.audible-dl.json`.

TODO
----
- **Concurrently download and convert books.** We've been doing this
  sequentially for prototyping's sake, but this is dumb.  We'll also
  have to re-work the status outputs to work with concurrent downloads
  and conversions
- **Add command-line options.** Stuff like -d to specify a different
  audiobooks directory and -a to convert a single book.
- **Clean up scraper.go** The Book struct stores a lot of information
  we're not using.
- **Automatically get activations bytes.**
