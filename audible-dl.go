package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// If the -l or --log flag is passed, in addition to logging to an
// internal buffer, the scraper will log to the file
// .audible-dl-debug.log.  The scraper runs synchronously so there
// should be no risk of race conditions causing the contents of this
// or the account buffers to be mangled
var logFile *os.File = nil

// Each book is stored in one of these
type Book struct {
	Slug         string   // B002VA9SWS
	Title        string   // "The Hitchhiker's Guide to the Galaxy"
	Series       string   // "The Hitchhiker's Guide to the Galaxy"
	Runtime      string   // "5 hrs and 51 minutes"
	Summary      string   // "Seconds before the Earth is demolished..."
	CoverURL     string   // "https://m.media-amazon.com/..."
	FileName     string   // "TheHitchhikersGuidetotheGalaxy"
	DownloadURL  string   // "https://cds.audible.com/..."
	CompanionURL string   // ""
	Authors      []string // ["Douglas Adams"]
	Narrators    []string // ["Steven Fry"]
	SeriesIndex  int      // 1
}

////////////////////////////////////////////////////////////////////////
//             _                          _       _
//   ___ _ __ | |_ _ __ _   _ _ __   ___ (_)_ __ | |_
//  / _ \ '_ \| __| '__| | | | '_ \ / _ \| | '_ \| __|
// |  __/ | | | |_| |  | |_| | |_) | (_) | | | | | |_
//  \___|_| |_|\__|_|   \__, | .__/ \___/|_|_| |_|\__|
//                      |___/|_|
////////////////////////////////////////////////////////////////////////

func main() {
	account, harpath, aaxpath, savelog := getArgs()
	cfgfile, datadir, tempdir, savedir := getPaths()
	client := MakeClient(cfgfile, tempdir, savedir, datadir)
	client.Validate()

	if savelog {
		var err error
		logFile, err = os.OpenFile(".audible-dl-debug.log",
			os.O_WRONLY|os.O_CREATE, 0644)
		expect(err, "Failed to open log file for writing")
	}

	if harpath != "" {
		client.ImportCookies(account, harpath)
		os.Exit(0)
	}

	if aaxpath != "" {
		m4b := client.ConvertSingleBook(account, aaxpath)
		fmt.Printf("%s: made %s\n", account, filepath.Base(m4b))
		os.Exit(0)
	}

	client.GetCookies()
	client.GetDownloaded()
	client.ScrapeLibrary(account)

	logFile.Close()
}

////////////////////////////////////////////////////////////////////////
//                   _ _ _            _
//   __ _ _   ___  _(_) (_) __ _ _ __(_) ___  ___
//  / _` | | | \ \/ / | | |/ _` | '__| |/ _ \/ __|
// | (_| | |_| |>  <| | | | (_| | |  | |  __/\__ \
//  \__,_|\__,_/_/\_\_|_|_|\__,_|_|  |_|\___||___/
////////////////////////////////////////////////////////////////////////

const helpMessage string = `Usage: audible-dl [-h] [-a ACC] [-i HAR] [-s AAX]

  Scrape your Audible library or convert an AAX file to m4b.
  See audible-dl(1) for more information.

Options:
  -h, --help         Print this message and exit.
  -a, --account NAME Specify an account for the operation.
  -i, --import  HAR  Import login cookies from HAR.
  -s, --single  AAX  Convert the single AAX file specified in AAX.
  -l, --log          Log scraper info to .audible-dl-debug.log
`

const debugScraperMessage string = `I encountered an error while scraping your library.
There are two likely causes of this:

  1. Your authentication cookies for %s have expired.  You can re-import
     them with "audible-dl -i path/to/cookies.har -a %s", see the man
     page for details.
  2. Audible changed the structure of their website.  This is most likely the
     case if the file .audible-dl-debug.html contains a list of books or
     otherwise looks like you were signed in correctly.

If re-importing your cookies doesn't help, please email a bug report to
"~thalia/audible-dl@lists.sr.ht", see the man page for details.
`

// Like Rust's .unwrap() method.
func unwrap(err interface{}) {
	if err != nil {
		log.Fatal(err)
	}
}

// Like Rust's .expect() method.
func expect(err interface{}, why string) {
	if err != nil {
		log.Print(why)
		log.Print(err)
		fmt.Fprintf(os.Stderr, helpMessage)
		os.Exit(1)
	}
}

////////////////////////////////////////////////////////////////////////
//                  _                                      _
//   ___ _ ____   _(_)_ __ ___  _ __  _ __ ___   ___ _ __ | |_
//  / _ \ '_ \ \ / / | '__/ _ \| '_ \| '_ ` _ \ / _ \ '_ \| __|
// |  __/ | | \ V /| | | | (_) | | | | | | | | |  __/ | | | |_
//  \___|_| |_|\_/ |_|_|  \___/|_| |_|_| |_| |_|\___|_| |_|\__|
////////////////////////////////////////////////////////////////////////

// Audible-dl needs to access several locations in the filesystem in
// order to save downloaded books and discover config, auth, and cache
// data.  Typically, it will look for internal files in the
// OS-specific cache and config directories with downloaded books
// stored in a location that the user must specify in the config
// file.  If $AUDIBLE_DL_ROOT points to a directory, all program state
// will live inside it.
func getPaths() (string, string, string, string) {
	var cfgfile, datadir, tempdir, savedir string
	root := os.Getenv("AUDIBLE_DL_ROOT")
	if root != "" {
		cfgfile = root + "/.audible-dl/config.yml"
		tempdir = root + "/.audible-dl/temp/"
		datadir = root + "/.audible-dl/"
		savedir = root + "/"
	} else {
		conf, err := os.UserConfigDir()
		unwrap(err)
		cach, err := os.UserCacheDir()
		unwrap(err)
		cfgfile = conf + "/audible-dl/config.yml"
		tempdir = cach + "/audible-dl/temp/"
		// FIXME: Ideally these would go in XDG_DATA_HOME but
		// golang doesn't have an os.UserDataDir() so we're
		// stricking them in with the config file instead.
		datadir = conf + "/audible-dl/"
		savedir = "" // Read later from config file
	}
	return cfgfile, datadir, tempdir, savedir
}

// Read command-line arguments.
func getArgs() (string, string, string, bool) {
	var a, h, s string
	var l bool
	// FIXME: prevent duplicate flags
	flag.StringVar(&a, "a", "", "")
	flag.StringVar(&h, "i", "", "")
	flag.StringVar(&s, "s", "", "")
	flag.BoolVar(&l, "l", false, "")
	flag.StringVar(&a, "account", "", "")
	flag.StringVar(&h, "import", "", "")
	flag.StringVar(&s, "single", "", "")
	flag.BoolVar(&l, "log", false, "")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, helpMessage)
	}
	flag.Parse()
	return a, h, s, l
}
