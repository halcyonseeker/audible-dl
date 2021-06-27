audible-dl --- An archiver for your Audible library
===================================================

**This program is slow and stupid. Important functionality like
concurrent downloads has not been implemented.**

Audible-dl is an archiving tool that keeps an up-to-date, offline,
archive of the audiobooks you've purchased with Audible as DRM-free,
metadata rich, opus files.

It is intended for individuals who wish to effortlessly maintain an
offline store of their Audiobooks to browse and listen to on their own
terms. Please do not use audible-dl to upload books to piracy sites;
authors and narrators need to make a living somehow.

Install
-------
Audible-dl depends on ffmpeg and the Go programming language. You
should be able to build it for any OS supported by the Go compiler,
however I've only tested it on Arch GNU/Linux and FreeBSD. If you're
running a Unix like operating system you can install audible-dl to
/usr/local/bin by running `make install` as root.

User Guide
----------
Audible-dl stores all your books in a single directory, `~/Audiobooks`
let's say.

1. Login to Audible in your browser, navigate to
   <https://audible.com/library/titles>, pop open inspect-element and
   manually copy over the request cookies into the follwing Json
   object in `~/Audiobooks/.audible-dl-cookies.json`:

    ```json
    [
        { "Name": "csm-hit", "Value": "" },
        { "Name": "session-id", "Value": "" },
        { "Name": "session-id-time", "Value": "" },
        { "Name": "ubid-main", "Value": "" }
    ]
	```

2. Acquire your activation bytes. There are a number of guides online
   of how to do it.

3. In the audiobooks directory, just run `audible-dl -b deadbeef`. If
   this is the first run it will download all your audiobooks, storing
   them in the current directory. Subsequent runs will download only
   those books that aren't present, so it's recommended that you don't
   change the file names

4. Enjoy!

TODO
----
- **Concurrently download and convert books.** We've been doing this
  sequentially for prototyping's sake, but this is dumb. We'll also
  have to re-work the status outputs to work with concurrent downloads
  and conversions
- **Add command-line options.** Stuff like -d to specify a different
  audiobooks directory.
- **Store configuration in a file.** I shouldn't have to provide my
  activation bytes every time.
- **Write cover image to opus.** ffmpeg's libopus module doesn't do
  this so we should do it automatically.
- **Make opus metadata more audiobook like.** Why does everyone assume
  audio files contain music -_-?
- **Implement login.** The cookies should last a few months, but
  manually inspect-elementing them into a json file is really
  tedious.
- **Clean up scraper.go** We're storing a lot of information we're not
  using.
- **Speed up near/far diff function** Iterating over slices to select
  the ones we need to download is horrifically slow. Perhaps the
  scraper should return a hash table instead of a []Book?
- **Automatically get activations bytes.**
- **Support multiple accounts.**
