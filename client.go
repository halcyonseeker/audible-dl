package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Escape code clear a line and move the cursor to the beginning
const clearline string = "\x1b[2k\r"

// Return S as bold text
func bold(s string) string {
	return "\033[1m" + s + "\033[m"
}

////////////////////////////////////////////////////////////////////////
//       _ _            _           _     _           _
//   ___| (_) ___ _ __ | |_    ___ | |__ (_) ___  ___| |_
//  / __| | |/ _ \ '_ \| __|  / _ \| '_ \| |/ _ \/ __| __|
// | (__| | |  __/ | | | |_  | (_) | |_) | |  __/ (__| |_
//  \___|_|_|\___|_| |_|\__|  \___/|_.__// |\___|\___|\__|
//                                     |__/
////////////////////////////////////////////////////////////////////////

// A single instance of the client struct is used to encapsulate all
// internal state.  SaveDir is where we're saving completed .m4b
// files, TempDir is where we're downloading .aax files to, and
// DataDir is where we look for cache and authentication files.
// Accounts is a slice of the accounts set up in the config file and
// Downloaded is map of all the books we've previously downloaded.
// This map is populated from a cache file which exists to allow the
// user to rename and organize their collection after they've been
// downloaded.
type Client struct {
	SaveDir    string
	TempDir    string
	DataDir    string
	Accounts   []Account
	Downloaded map[string]Book
}

// Return a Client struct partially populated from the .yml file
// passed in CFGFILE.
func MakeClient(cfgfile, tempdir, savedir, datadir string) Client {
	var client Client
	client.Downloaded = make(map[string]Book)
	raw, err := os.ReadFile(cfgfile)
	expect(err, "Please create the config file with at least one account")
	expect(yaml.Unmarshal(raw, &client), "Bad yaml in config file")
	client.TempDir = tempdir
	client.DataDir = datadir
	if os.Getenv("AUDIBLE_DL_ROOT") != "" {
		client.SaveDir = savedir
	}
	os.MkdirAll(tempdir, 0755)
	os.MkdirAll(savedir, 0755)
	return client
}

// Make sure everything in the client is set up correctly.
func (c *Client) Validate() {
	if c.SaveDir == "" {
		log.Fatal("savdir not specified on config file")
	}
	if c.TempDir == "" {
		panic("Failed to infer TempDir!")
	}
	if len(c.Accounts) == 0 {
		log.Fatal("Couldn't find any accounts in config file.")
	}
	for _, a := range c.Accounts {
		if a.Name == "" {
			log.Fatal("Account name not specified in config file.")
		}
		if a.Bytes == "" {
			log.Fatal("Activation bytes not present for account " +
				a.Name)
		}
		// It's okay not to have cookies
	}
}

// Given an account name (likely passed with -a on the command line),
// return a pointer to the corresponding Account struct.
func (c *Client) FindAccount(name string) *Account {
	for _, a := range c.Accounts {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

// Given an account name (likely passed with -a on the command line),
// make sure it exists.  If an empty string is passed and there are
// more than one accounts set up or if the requested account doesn't
// exist, throw an error.
func (c *Client) NeedAccount(account string) (string, error) {
	if len(c.Accounts) != 1 && account == "" {
		return "", errors.New(
			"You have multiple accounts set up, please specify one")
	} else if account == "" {
		return c.Accounts[0].Name, nil
	} else if len(c.Accounts) > 1 && account != "" {
		if c.FindAccount(account) == nil {
			return "", errors.New(
				"Account " + account + " doesn't exist")
		}
	}
	return account, nil
}

// Given a .har file containing a full archive of a GET request to
// audible.com/library/titles in HARPATH, import the cookies therein
// into ACCOUNT's cookie store.
func (c *Client) ImportCookies(account, harpath string) {
	account, err := c.NeedAccount(account)
	authpath := c.DataDir + account + ".cookies.json"
	unwrap(err)
	a := c.FindAccount(account)
	raw, err := ioutil.ReadFile(harpath)
	unwrap(err)
	a.ImportCookiesFromHAR(raw)
	json, _ := json.MarshalIndent(a.Auth, "", "  ")
	unwrap(ioutil.WriteFile(authpath, json, 0644))
	fmt.Printf("Imported cookies from %s into %s\n", harpath, authpath)
}

// Using ACCOUNT's bytes, convert a single .aax file passed in AAXPATH
// and return the path of the created .m4b file.
func (c *Client) ConvertSingleBook(account string, aaxpath string) string {
	account, err := c.NeedAccount(account)
	unwrap(err)
	a := c.FindAccount(account)
	var m4bpath string
	if aaxpath[len(aaxpath)-4:] == ".aax" {
		m4bpath = aaxpath[:len(aaxpath)-4] + ".m4b"
	} else {
		m4bpath = aaxpath + ".m4b"
	}
	err, ffmpegstderr := a.Convert(aaxpath, m4bpath, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ffmpegstderr)
		log.Fatalf("Failed to convert %s with bytes %s\n",
			filepath.Base(aaxpath), a.Bytes)
	}
	return m4bpath
}

// For each account, load the cached cookies into memory.
func (c *Client) GetCookies() {
	for i := 0; i < len(c.Accounts); i++ {
		a := &c.Accounts[i]
		if !a.Scrape {
			continue
		}
		path := c.DataDir + a.Name + ".cookies.json"
		raw, err := os.ReadFile(path)
		expect(err, "Couldn't find any cookies for account "+a.Name)
		expect(json.Unmarshal(raw, &a.Auth),
			"Unknown json in cookie file for account "+a.Name)
	}
}

// Populate client's hash table of previously downloaded books from a
// json file.
func (c *Client) GetDownloaded() {
	var books []Book
	path := c.DataDir + "downloaded_books.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		// It's okay for the file not to exist
		if !os.IsNotExist(err) {
			log.Fatal(err)
		}
		return
	}
	expect(json.Unmarshal(raw, &books), "Bad json in downloaded book file")
	for _, b := range books {
		c.Downloaded[b.Title] = b
	}
}

// Write the map of downloaded books off to the file, overwriting its
// old contents
func (c *Client) SetDownloaded() {
	var books []Book
	for _, b := range c.Downloaded {
		books = append(books, b)
	}
	json, _ := json.MarshalIndent(books, "", "  ")
	unwrap(ioutil.WriteFile(c.DataDir+"downloaded_books.json", json, 0644))
}

// This function orchestrates the scraping, downloading, and
// conversion of audiobooks for all configured acounts or the one
// passed in ACCOUNT.  It also displays a progress report in stdout.
func (c *Client) ScrapeLibrary(account string) {
	var toscrape []Account
	if a := c.FindAccount(account); a != nil {
		toscrape = append(toscrape, *a)
		if !a.Scrape {
			log.Fatalf("Account %s has `scrape' set to false.",
				a.Name)
		}
	} else {
		toscrape = c.Accounts
	}
	for _, a := range toscrape {
		if !a.Scrape {
			continue
		}
		for i := 0; i < len(books); i++ {
			b := books[i]
			if _, ok := c.Downloaded[b.Title]; ok {
				continue
			}
			fmt.Printf("\033[1mDownloading Book\033[m %s...", b.Title)
			aax := a.DownloadSingleBook(c, b)
			fmt.Printf("done\n")
			fmt.Printf("\033[1mConverting Book\033[m %s...", b.Title)
			m4b := c.ConvertSingleBook(a.Name, aax)
			unwrap(os.Rename(m4b, c.SaveDir+b.FileName+".m4b"))
			fmt.Printf("done\n")
			c.Downloaded[b.Title] = b
			c.SetDownloaded()
		}
	}
}

func scrapeLibraryWithPrinting(a *Account) []Book {
	var wg sync.WaitGroup
	ch := make(chan int)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var npages int
		for i := range ch {
			npages = i
			fmt.Printf("%s%s %d", clearline, bold("Scraping Page"),
				npages)
		}
		fmt.Printf("%s%s %d/%d\n", clearline, bold("Scraped Page"),
			npages, npages)
	}()
	books, err := a.ScrapeFullLibrary(ch)
	wg.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "BEGIN SCRAPER LOG\n")
		a.PrintScraperDebuggingInfo()
		fmt.Fprintf(os.Stderr, "END SCRAPER LOG\n")
		log.Println(err)
		fmt.Fprintf(os.Stderr, debugScraperMessage, a.Name, a.Name)
	}
	return books
}
