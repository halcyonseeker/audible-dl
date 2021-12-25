//
// audible-dl.go --- A simple tool for archiving your audible library
// sr.ht/~thalia/audible-dl
// Thalia Wright <ymir@ulthar.xyz>
//

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"git.sr.ht/~thalia/audible-dl/audibledl"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
)

const noDataFileError string = `Data file is not present.  Create it by running:

	audible-dl -b [bytes] -i [file.har]

For more information or to report a bug, see the project website:
	https://sr.ht/~thalia/audible-dl
`

const scraperFailedError string = `Received this error scrapping your library:

	%s

This could mean:
1. Your computer isn't connected to the internet.
2. Your cached Audible cookies have expired.  If you think that this might
   be the case, get a new HAR file and try re-importing them with:

	audible-dl -i [file.har]

3. If the previous step fails then Audible probably changed the structure
   of their website; see the file .audible-dl-debug.html and please email a
   report to ~thalia/audible-dl@lists.sr.ht
`

// Download the audiobook passed as an argument, saving it as an aax file
// in the cache directory
func downloadSingleBook(b *audibledl.Book, cfg *audibledl.Client) error {
	aax := cfg.Cache + b.FileName + ".aax"

	// Append .part to differentiate partially downloaded files
	out, err := os.Create(aax + ".part")
	if err != nil {
		return err
	}

	// Interestingly, the downlaod URLs are publicly accessible
	resp, err := http.Get(b.DownloadURL)
	if err != nil {
		return err
	} else {
		if resp.StatusCode != http.StatusOK {
			return errors.New("Request returned " + resp.Status)
		}
	}

	nbytes, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	} else {
		if nbytes != resp.ContentLength {
			return errors.New("Failed to write file to disk")
		}
	}

	// Rename the file now that it has been fully downloaded
	if err := os.Rename(aax+".part", aax); err != nil {
		return err
	}

	return nil
}

// Helper function to reduce the nesting in main
func fileExists(fn string) bool {
	if _, err := os.Stat(fn); err != nil {
		return !os.IsNotExist(err)
	}
	return true
}

// Write an ADLData struct to ./.audible-dl.json.  This should probably
// only be done when the user first runs this program with the -i and -b
// flags. Subsequent runs should just read the required data from the file.
func writeDataFile(cfg *audibledl.Client) error {
	json, _ := json.MarshalIndent(cfg, "", "  ")
	if err := ioutil.WriteFile(".audible-dl.json", json, 0644); err != nil {
		return err
	}

	return nil
}

func main() {
	var cfg audibledl.Client
	var abytes, harpath, aaxpath string

	flag.StringVar(&abytes, "b", "", "Your Audible activation bytes.")
	flag.StringVar(&harpath, "i", "", "Import a HAR file.")
	flag.StringVar(&aaxpath, "a", "", "Convert a single AAX file.")
	flag.Parse()

	// If the cache file exists, read it, otherwise check if we're
	// only converting a single book and exit if not
	if fileExists(".audible-dl.json") {
		raw, err := ioutil.ReadFile(".audible-dl.json")
		if err != nil {
			log.Fatal(err)
		}
		cfg.InitFromJson(raw)
	} else if aaxpath == "" {
		fmt.Printf(noDataFileError)
		os.Exit(1)
	}

	// If we're only converting a single book just convert it and
	// exit without writing to the data file
	if aaxpath != "" {
		if cfg.Bytes == "" && abytes == "" {
			log.Fatal("Please pass activation bytes with -a.")
		}
		if cfg.Bytes == "" || abytes != "" {
			cfg.Bytes = abytes
		}
		if aaxpath[len(aaxpath)-4:] == ".aax" {
			aaxpath = aaxpath[:len(aaxpath)-4]
		}
		fmt.Printf("\033[1mConverting\033[m %s...", aaxpath)
		if err := cfg.Convert(aaxpath); err != nil {
			fmt.Printf("\n")
			log.Fatal(err)
		}
		fmt.Printf("ok\n")
		os.Exit(0)
	}

	if abytes != "" {
		cfg.Bytes = abytes
		if err := writeDataFile(&cfg); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\033[1m Set Activation Bytes to\033[m %s\n", abytes)
	}

	if harpath != "" {
		raw, err := ioutil.ReadFile(harpath)
		if err != nil {
			log.Fatal(err)
		}
		if err := cfg.ImportCookiesFromHAR(raw); err != nil {
			log.Fatal(err)
		}
		if err := writeDataFile(&cfg); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\033[1mImported Cookies From\033[m %s\n", harpath)
	}

	// Create cache dir to store partially downloaded audiobook files.
	usercache, err := os.UserCacheDir()
	if err != nil {
		log.Fatal(err)
	}
	cfg.Cache = usercache + "/audible-dl/"
	if err := os.Mkdir(cfg.Cache, 0755); err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
	}

	// Scrape a list of all your audiobooks from audible's website then
	// download and convert the ones that don't appear to be present in
	// the current working directory.

	ch := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var t int
		for i := range ch {
			t = i
			fmt.Printf("\x1b[2k\r\033[1mScraping Page\033[m %d", i)
		}
		fmt.Printf("\x1b[2k\r\033[1mScraped Page\033[m %d/%d\n", t, t)
	}()
	books, err := cfg.ScrapeFullLibrary(ch)
	wg.Wait()
	if err != nil {
		fmt.Printf(scraperFailedError, err)
		os.Exit(1)
	}

	// Download and convert the books
	for _, b := range books {
		if !fileExists(b.FileName + ".m4b") {
			if b.DownloadURL == "" {
				fmt.Printf("\033[1mNo URL for\033[m %s\n", b.Title)
				continue
			}
			fmt.Printf("\033[1mDownloading\033[m %s...", b.Title)
			if err := downloadSingleBook(&b, &cfg); err != nil {
				fmt.Printf("\n")
				log.Fatal(err)
			}
			fmt.Printf("ok\n")
			fmt.Printf("\033[1mConverting\033[m %s...", b.Title)
			if err := cfg.Convert(b.FileName); err != nil {
				fmt.Printf("\n")
				log.Fatal(err)
			}
			fmt.Printf("ok\n")
			err := os.Remove(cfg.Cache + b.FileName + ".aax")
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
