audible-dl: An archiver for your Audible library
================================================

**Audible-dl is functional per the documentation but there are still
improvements to be made and kinks to be ironed out.**

Audible-dl is an archiving tool that keeps an up-to-date, offline,
archive of the audiobooks you've purchased with Audible as DRM-free,
metadata rich, m4b files.

Install
=======
Audible-dl depends on ffmpeg, and the Go programming language.  You
should be able to build it for any OS supported by the Go compiler,
however I've only tested it on Arch GNU/Linux and FreeBSD. Build it
with `make` and install or uninstall it by running `make install` or
`make uninstall` as root.

Usage
=====
See the man page `man 1 audible-dl` for usage details, or check out
the [markdown version](./audible-dl.1.md).

Bugs, Help, and Contributions
=============================
Please report bugs, request help, and contribute patches by emailing
[~thalia/audible-dl@lists.sr.ht](mailto:~thalia/audible-dl@lists.sr.ht).

TODO
====
- General code cleanup.
- Implement asynchronous downloading.
- Add some config options, primarily to give the user control over how
  the final files are named.
- Fix "403 forbidden" error when downloading some books.
- `Account.ScrapeLibraryUntil()`'s functionality isn't really being
  used; add a flag and config option to take advantage of it in order
  to reduce the time spent scraping the user's library.
- Figure out a way of automatically finding activations bytes, maybe
  by integrating with the Rainbow Tables plugin.
- When importing cookies, if there are multiple accounts in the config
  file and only one with `scrape: true` (or without `scrape: false`),
  default to that instead of returning an error.
