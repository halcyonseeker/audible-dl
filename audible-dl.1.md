```
AUDIBLE-DL(1)             BSD General Commands Manual            AUDIBLE-DL(1)

NAME
     audible-dl — A downloader for your Audible audiobook library.

SYNOPSIS
     audible-dl [-h, --help] [-l, --log] [-a, --account account]
                [-i, --import file.har] [-s, --single file.aax]

DESCRIPTION
     audible-dl is a simple command-line utility to create offline archives of
     your Audible library as DRM-free .m4b files, wrapping ffmpeg(1) in order
     to crack Audible's DRM.  It supports multiple Audible accounts and can be
     used to convert pre-downloaded .aax files.

   Prerequisites
     In order for audible-dl to be useful, you need two things.  Firstly, a
     HAR (HTTP Archive Format) file containing your Audible authentication
     cookies which we use to scrape your account.  You can get this by logging
     into https://audible.com in your browser, opening the network tab of the
     element inspector, then navigating to https://audible.com/library/titles
     and right-clicking on the GET request to that page, selecting "Copy All
     As HAR".  This can then be pasted into a file.  Secondly, your Audible
     activation bytes which are required to crack the DRM on .aax files.
     There are multiple ways to get them, the one I had the most luck with was
     a dedicated plugin for RainbowCrack; just follow the instructions on
     https://github.com/inAudible-NG/tables.

     Technically, only the second step is required if you just want to convert
     .aax files to .m4b files, though at that point you might as well just
     write a quick ffmpeg(1) script instead.

     Once you have your activation bytes and authentication cookies, add the
     former to your config file (see audible-dl EXAMPLES) and import the lat‐
     ter with:

         audible-dl -i path/to/cookies.har

   Options
     audible-dl supports a few command-line options:

     -h, --help
         Display a quick help message.

     -l, --log
         Save a log of the scraper's progress into the file
         .audible-dl-debug.log.  The contents of this file may be useful in
         bug reports.

     -a, --account account
         Some operations like converting a single .aax file or importing au‐
         thentication cookies from a .har file require that you specify an ac‐
         count with which to perform the operation.  The argument should be
         the account's name field in the config file.  This option may be
         omitted if you have only one account set up.

     -i, --import path/to/file.har
         Import authentication cookies from a HAR archive into the specified
         account.

     -s, --single path/to/file.aax
         Convert a single .aax file into an .m4b file using the specified ac‐
         count.

   Configuration
     In order to use audible-dl a YAML config file must be created.  At the
     very minimum it must contain a list named accounts where each entry con‐
     tains at least a name and a bytes field.  If AUDIBLE_DL_ROOT is unset the
     file should also contain a savedir field specifying the directory in
     which to save downloaded audiobooks.  If that variable is set and savedir
     specifies a directory, then books are saved into that directory rather
     than the one pointed to by the variable.

     More config options may be added in the future, including naming rules
     for downloaded books and the ability to specify things like the savedir
     on a per-account basis.

ENVIRONMENT
     AUDIBLE_DL_ROOT
         When set to an existing directory, tell audible-dl to look for all of
         its state beneath it.  Downloaded books will be saved there and tem‐
         porary and system files will be stored in the .audible-dl/ subdirec‐
         tory.

     XDG_CONFIG_HOME
         By default, audible-dl looks for its config file, authentication
         cookies, and list of downloaded books in the audible-dl/ subdirec‐
         tory.  The appropriate configuration directory is inferred using
         Golang's os.UserConfigDir() function which will return something com‐
         pletely different on Mac OS, Windows, and Plan 9.

     XDG_CACHE_HOME
         By default, audible-dl stores temporary intermediate files in the
         audible-dl/temp/ directory.  The appropriate cache directory is in‐
         ferred using Golang's os.UserCacheDir() function and will return dif‐
         ferent values on Mac OS, Windows, and Plan 9.

FILES
     config.yml
         The core configuration file.

     downloaded_books.json
         A list of the books that have already been downloaded.  This file al‐
         lows the you to organize and rename your audiobooks at your leisure.

     [name].cookies.json
         Each account's authentication cookies.

EXAMPLES
   Average use-case
     Most users will likely want to use audible-dl to download books in its
     default run mode with AUDIBLE_DL_ROOT unset.  For someone with a single
     account, their config file, ~/.config/audible-dl/config.yml, will look
     like this:

         savedir: "~/Audiobooks/Audible/"
         accounts:
           - name: "Personal"
             bytes: "deadbeef"

     After running and ~/Audiobooks/Audible/ will contain all their books as
     .m4b files ready to be transferred to a phone or mp3 player or indexed
     into an audiobook library browser.

   Keeping all state in a single directory
     I like to synchronize my media between several machines, so my preference
     is to keep everything in a single directory.  In my shell's rc file I
     have:

         export AUDIBLE_DL_ROOT="$HOME/media/audiobookes/audible"

     In ~/media/audiobooks/audible/.audible-dl/config.yml I have:

         accounts:
           - name: "Personal"
             bytes: "deadbeef"
           - name: "Other"
             bytes: "beefdead"

     Note that I have two accounts set up.

SEE ALSO
     ffmpeg(1) ffprobe(1)

     tables, https://github.com/inAudible-NG/tables.

     Cozy, https://cozy.sh.

     Voice, https://github.com/PaulWoitaschek/Voice.

     OpenAudible, https://openaudible.org.

     audiobookshelf, https://www.audiobookshelf.org.

AUTHORS
     ꙮ <ymir@ulthar.xyz>

HOME
     https://sr.ht/~thalia/audible-dl

BUGS
     As of the writing of this I am not aware of any bugs.  If you find any,
     most likely due to a change in Audible's website, please report them by
     sending a detailed email to ~thalia/audible-dl@lists.sr.ht.  If possible,
     please attach the .audible-dl-debug.html and .audible-dl-debug.log files
     as well as your config file, cookie file(s), and downloaded books file
     with all personal info censored.

SECURITY CONSIDERATIONS
     audible-dl stores your Audible authentication cookies in plain-text json
     files.  This means that an attacker who gains access to them will be able
     to log into your Audible account in the browser.  Ideally, we wouldn't
     have to manage sensitive data ourselves and would simply source your
     username and password from your system's keychain, but I've found Audi‐
     ble's login process to be too complex to easily reverse engineer.

BSD                              July 7, 2022                              BSD
```
