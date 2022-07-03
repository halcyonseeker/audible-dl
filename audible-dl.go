package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// If the -l or --log flag is passed, in addition to logging to an
// internal buffer, the scraper will log to the file
// .audible-dl-debug.log.  The scraper runs synchronously so there
// should be no risk of race conditions causing the contents of this
// or the account buffers to be mangled
var logFile *os.File = nil

////////////////////////////////////////////////////////////////////////
//       _ _            _           _     _           _
//   ___| (_) ___ _ __ | |_    ___ | |__ (_) ___  ___| |_
//  / __| | |/ _ \ '_ \| __|  / _ \| '_ \| |/ _ \/ __| __|
// | (__| | |  __/ | | | |_  | (_) | |_) | |  __/ (__| |_
//  \___|_|_|\___|_| |_|\__|  \___/|_.__// |\___|\___|\__|
//                                     |__/
////////////////////////////////////////////////////////////////////////

type Client struct {
	SaveDir    string
	TempDir    string
	Accounts   []Account
	Downloaded map[string]Book
}

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

func (c *Client) FindAccount(name string) *Account {
	for _, a := range c.Accounts {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////
//                                  _           _     _           _
//   __ _  ___ ___ ___  _   _ _ __ | |_    ___ | |__ (_) ___  ___| |_
//  / _` |/ __/ __/ _ \| | | | '_ \| __|  / _ \| '_ \| |/ _ \/ __| __|
// | (_| | (_| (_| (_) | |_| | | | | |_  | (_) | |_) | |  __/ (__| |_
//  \__,_|\___\___\___/ \__,_|_| |_|\__|  \___/|_.__// |\___|\___|\__|
//                                                 |__/
////////////////////////////////////////////////////////////////////////

type Account struct {
	Name   string
	Bytes  string
	Auth   []*http.Cookie
	Scrape bool
	LogBuf bytes.Buffer
}

func (a *Account) String() string {
	ret := "Account:\n"
	ret += "  Name:   " + a.Name + "\n"
	ret += "  Bytes:  " + a.Bytes + "\n"
	ret += "  Scrape: " + strconv.FormatBool(a.Scrape) + "\n"
	ret += "  Auth:\n"
	for _, c := range a.Auth {
		ret += "    " + c.Name + ": " + c.Value + "\n"
	}
	return ret
}

// Flush the contents of a.Log to stderr in order to make it easier to
// debug the scraper's progress.
func (a *Account) PrintScraperDebuggingInfo() {
	fmt.Fprintln(os.Stderr, a.LogBuf.String())
}

// Log the scraper's debugging info to the internal buffer to be
// printed by the above method.
func (a *Account) Log(str string, args ...any) {
	line := a.Name + ": " + fmt.Sprintf(str, args...) + "\n"
	a.LogBuf.WriteString(line)
	if logFile != nil {
		_, err := logFile.WriteString(line)
		unwrap(err)
	}
}

func (a *Account) ImportCookiesFromHAR(raw []byte) {
	var har map[string]interface{}

	unwrap(json.Unmarshal(raw, &har))

	cookies := har["log"].(map[string]interface{})["entries"].([]interface{})[0].(map[string]interface{})["request"].(map[string]interface{})["cookies"].([]interface{})

	for _, c := range cookies {
		value := c.(map[string]interface{})["value"].(string)
		// The values of some non-essential cookies contain a double
		// quote character which net/http really doesn't like
		if strings.Contains(value, "\"") {
			continue
		}
		a.Auth = append(a.Auth, &http.Cookie{
			Name:  c.(map[string]interface{})["name"].(string),
			Value: value,
		})
	}
}

func (a *Account) Convert(in, out string, client Client) (error, []byte) {
	tmp := client.TempDir + filepath.Base(out)
	cmd := exec.Command("ffmpeg",
		"-activation_bytes", a.Bytes,
		"-i", in,
		"-c", "copy",
		tmp)
	cmd.Stdout = nil
	stderr, _ := cmd.StderrPipe()
	err := cmd.Start()
	slurp, _ := io.ReadAll(stderr)
	if err != nil {
		return err, slurp
	}
	if err = cmd.Wait(); err != nil {
		return err, slurp
	}
	if err = os.Rename(tmp, out); err != nil {
		return err, nil
	}
	return nil, nil
}

// Download a HTMl page in the user's library
func (a *Account) getLibraryPage(page int) ([]byte, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	uri := "https://www.audible.com/library/titles?page=" + strconv.Itoa(page)
	req, _ := http.NewRequest("GET", uri, nil)

	jaruri, _ := url.ParseRequestURI(uri)
	jar.SetCookies(jaruri, a.Auth)

	a.Log("Fetching library page %d", page)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New("getLibraryPage: " + resp.Status)
	}

	html, _ := ioutil.ReadAll(resp.Body)
	return html, nil
}

// Scrape the library until we encounter a book whose filename
// (display title ran through stripstr) matches lim, returning a slice
// of books.  If lim is an empty string this behaves exactly like
// ScrapeFullLibrary().
func (a *Account) ScrapeLibraryUntil(pagenum chan int, lim string) ([]Book, error) {
	var books []Book

	defer close(pagenum)

	// audible.com/library/titles?page=N doesn't return a 404 when
	// we access pages that don't exist, so we'll store the slug of
	// the first books of the current and previous pages in these
	// respective variables and check against the current book's slug.
	var firstincurrpage string = ""
	var firstinprevpage string = ""

	for i := 1; ; i++ {
		pagenum <- i
		raw, err := a.getLibraryPage(i)
		if err != nil {
			return nil, err
		}

		a.Log("Tokenizing page %d", i)

		dom := html.NewTokenizer(bytes.NewReader(raw))

		for {
			tt := dom.Next()
			tok := dom.Token()
			if tokBeginsBook(tt, tok) {
				a.Log("Found a book row")
				// If we find a book, extract it
				book := a.xSingleBook(dom, tt, tok)
				if book.Slug == firstinprevpage {
					a.Log("Reached a duplicate page")
					return books, nil
				}
				if book.FileName == lim && lim != "" {
					a.Log("Reached the final book")
					return books, nil
				}
				books = append(books, book)
				if firstincurrpage == "" {
					// Save the first book in the page
					firstincurrpage = book.Slug
				}
				if firstinprevpage == "" {
					// This is the first page
					firstinprevpage = book.Slug
				}
				continue
			}

			// exit inner loop when we reach the end end
			if id(tok) == "center-6" || tt == html.ErrorToken {
				break
			}
		}
		// We're fetching the next page, so we cycle these out
		firstinprevpage = firstincurrpage
		firstincurrpage = ""

		// If we didn't extract any books from the first page then
		// we probably won't extract any from the next, so we should
		// just break and return an error, saving the page source in
		// a file along with debugging information
		if len(books) == 0 {
			ioutil.WriteFile(".audible-dl-debug.html", raw, 0644)
			return nil, errors.New("Failed to extract books from HTML")
		}
	}
}

func (a *Account) ScrapeFullLibrary(pagenum chan int) ([]Book, error) {
	return a.ScrapeLibraryUntil(pagenum, "")
}

////////////////////////////////////////////////////////////////////////
//                                             _   _ _ _ _
//  ___  ___ _ __ __ _ _ __   ___ _ __   _   _| |_(_) (_) |_ ___  ___
// / __|/ __| '__/ _` | '_ \ / _ \ '__| | | | | __| | | | __/ _ \/ __|
// \__ \ (__| | | (_| | |_) |  __/ |    | |_| | |_| | | | ||  __/\__ \
// |___/\___|_|  \__,_| .__/ \___|_|     \__,_|\__|_|_|_|\__\___||___/
//                    |_|
////////////////////////////////////////////////////////////////////////

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

// Determine if the current html token contains a book
func tokBeginsBook(tt html.TokenType, tok html.Token) bool {
	return tt == html.StartTagToken &&
		class(tok) == "adbl-library-content-row"
}

// Remove whitespace and other shell reserve characters from S
func stripstr(s string) string {
	r := regexp.MustCompile(
		`\s|\\|\(|\)|\[|\]|\{|\}|\*|\?|\!|\+|\,|\;|\&|\||\'|\"|‘`)
	s = r.ReplaceAllString(s, "")

	return s
}

// Remove extra whitespace and newlines from a string
func cleanstr(s string) string {
	r := regexp.MustCompile(`\s+`)
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\n")
	s = r.ReplaceAllString(s, " ")

	return s
}

// Get the class attribute of html tag T
func class(t html.Token) string {
	for _, a := range t.Attr {
		if a.Key == "class" {
			return cleanstr(a.Val)
		}
	}
	return ""
}

// Get the href attribute of html tag T
func href(t html.Token) string {
	for _, a := range t.Attr {
		if a.Key == "href" {
			return cleanstr(a.Val)
		}
	}
	return ""
}

// Get the id attribute of html tag T
func id(t html.Token) string {
	for _, a := range t.Attr {
		if a.Key == "id" {
			return cleanstr(a.Val)
		}
	}
	return ""
}

// Get the aria-label (how is this different from id??) attriute of hmtl tag T
func aria_label(t html.Token) string {
	for _, a := range t.Attr {
		if a.Key == "aria-label" {
			return cleanstr(a.Val)
		}
	}
	return ""
}

// Get the book's author and narrator (translator?)
func xPeople(dom *html.Tokenizer) []string {
	var people []string
	var prevtag string
	var pprevtag string
	tt := dom.Next()
	tok := dom.Token()

	for tok.Data != "li" {
		tt = dom.Next()
		tok = dom.Token()

		if tt == html.StartTagToken {
			pprevtag = prevtag
			prevtag = tok.Data
		}

		if tt == html.EndTagToken {
			pprevtag = prevtag
			prevtag = ""
		}

		if pprevtag == "a" && prevtag == "span" && tt == html.TextToken {
			people = append(people, cleanstr(tok.Data))
		}
	}
	return people
}

// Get the book's summary
func xSummary(dom *html.Tokenizer, tt html.TokenType, tok html.Token) string {
	var s string
	for !(tt == html.EndTagToken && tok.Data == "span") {
		tt = dom.Next()
		tok = dom.Token()

		if tt == html.TextToken {
			s += " " + tok.Data + " "
			s = cleanstr(s)
		}
	}
	return s
}

// Get the book's series name and index number
func xSeries(dom *html.Tokenizer, tt html.TokenType, tok html.Token) (string, int) {
	var series string
	var index int
	for !(tt == html.EndTagToken && tok.Data == "span") {
		tt = dom.Next()
		tok = dom.Token()
		if tt == html.TextToken {
			if series == "" {
				series = cleanstr(tok.Data)
			} else if index == 0 {
				s := cleanstr(tok.Data)
				if strings.Contains(s, ", Book") {
					index, _ = strconv.Atoi(s[len(s)-1:])
				}
				break
			}
		}
	}
	return series, index
}

// Fill in the structure for a single book
func (a *Account) xSingleBook(dom *html.Tokenizer, tt html.TokenType, tok html.Token) Book {
	var book Book

	// First we'll extract the book's slug from its div's id
	slug := id(tok)
	book.Slug = slug[len(slug)-10:]

	a.Log("Extracting a single book...")

	for {
		tt = dom.Next()
		tok = dom.Token()

		if strings.Contains(class(tok), "bc-image-inset-border") {
			for _, a := range tok.Attr {
				if a.Key == "src" {
					book.CoverURL = cleanstr(a.Val)
				}
			}
			a.Log("Found cover image URL: %s", book.CoverURL)
		} else if class(tok) == "bc-text bc-size-headline3" {
			tt = dom.Next()
			tok = dom.Token()
			book.Title = cleanstr(tok.Data)
			a.Log("Found book title: %s", book.Title)
			continue
		} else if strings.Contains(class(tok), "authorLabel") {
			book.Authors = xPeople(dom)
			a.Log("Found book author(s): %s", book.Authors)
			continue
		} else if strings.Contains(class(tok), "narratorLabel") {
			book.Narrators = xPeople(dom)
			a.Log("Found book narrator(s): %s", book.Narrators)
			continue
		} else if strings.Contains(class(tok), "merchandisingSummary") {
			book.Summary = xSummary(dom, tt, tok)
			a.Log("Found book summary: %s...", book.Summary[:10])
			continue
		} else if id(tok) == "time-remaining-display-"+book.Slug {
			for !(tt == html.EndTagToken && tok.Data == "span") {
				tt = dom.Next()
				tok = dom.Token()
				if tt == html.TextToken {
					book.Runtime = cleanstr(tok.Data)
				}
			}
			a.Log("Found book runtime: %s", book.Runtime)
			continue
		} else if strings.Contains(href(tok), "/series/") {
			book.Series, book.SeriesIndex = xSeries(dom, tt, tok)
			continue
			a.Log("Found book series and index: %s, %d",
				book.Series, book.SeriesIndex)
		} else if !strings.Contains(aria_label(tok), "DownloadPart ") &&
			strings.Contains(href(tok), "cds.audible.com/download") {
			book.DownloadURL = href(tok)
			a.Log("Found book download URL: %s", book.DownloadURL)
			continue
		} else if href(tok) == "/companion-file/"+book.Slug {
			book.CompanionURL = "https://audible.com" + cleanstr(href(tok))
			a.Log("Found book companion UR: %s", book.CompanionURL)
			continue
		}

		// We've arrived at the next boo
		if strings.Contains(class(tok), "library-item-divider") ||
			id(tok) == "adbl-library-content-toast-messaging" {
			a.Log("Breaking to next book")
			break
		}
	}
	book.FileName = stripstr(book.Title)
	return book
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
	client := getData(cfgfile, tempdir, savedir)
	client.Validate()

	log.Println("ACCOUNT", account)
	log.Println("HARPATH", harpath)
	log.Println("AAXPATH", aaxpath)
	log.Println("")
	log.Println("cfgfile", cfgfile)
	log.Println("authdir", authdir)
	log.Println("tempdir", tempdir)
	log.Println("savedir", savedir)
	log.Println("")
	log.Println(client)
	log.Println("")
	if savelog {
		var err error
		logFile, err = os.OpenFile(".audible-dl-debug.log",
			os.O_WRONLY|os.O_CREATE, 0644)
		expect(err, "Failed to open log file for writing")
	}

	if harpath != "" {
		doImportCookies(client, account, harpath, datadir)
		os.Exit(0)
	}

	if aaxpath != "" {
		doConvertSingleBook(client, account, aaxpath)
		os.Exit(0)
	}

	getCookies(client, datadir)
	getDownloaded(client, datadir)
	doScrapeLibrary(client, account)

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

func unwrap(err interface{}) {
	if err != nil {
		log.Fatal(err)
	}
}

func expect(err interface{}, why string) {
	if err != nil {
		log.Print(why)
		log.Print(err)
		fmt.Fprintf(os.Stderr, helpMessage)
		os.Exit(1)
	}
}

func needAccount(client Client, account string) (string, error) {
	if len(client.Accounts) != 1 && account == "" {
		return "", errors.New(
			"You have multiple accounts set up, please specify one")
	} else if account == "" {
		return client.Accounts[0].Name, nil
	} else if len(client.Accounts) > 1 && account != "" {
		if client.FindAccount(account) == nil {
			return "", errors.New(
				"Account " + account + " doesn't exist")
		}
	}
	return account, nil
}

////////////////////////////////////////////////////////////////////////
//                  _                                      _
//   ___ _ ____   _(_)_ __ ___  _ __  _ __ ___   ___ _ __ | |_
//  / _ \ '_ \ \ / / | '__/ _ \| '_ \| '_ ` _ \ / _ \ '_ \| __|
// |  __/ | | \ V /| | | | (_) | | | | | | | | |  __/ | | | |_
//  \___|_| |_|\_/ |_|_|  \___/|_| |_|_| |_| |_|\___|_| |_|\__|
////////////////////////////////////////////////////////////////////////

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

func getData(cfgfile, tempdir, savedir string) Client {
	var client Client
	raw, err := os.ReadFile(cfgfile)
	expect(err, "Please create the config file with at least one account")
	expect(yaml.Unmarshal(raw, &client), "Bad yaml in config file")
	client.TempDir = tempdir
	if os.Getenv("AUDIBLE_DL_ROOT") != "" {
		client.SaveDir = savedir
	}
	return client
}

func getCookies(client Client, datadir string) {
	for i := 0; i < len(client.Accounts); i++ {
		a := &client.Accounts[i]
		if !a.Scrape {
			continue
		}
		path := datadir + a.Name + ".cookies.json"
		raw, err := os.ReadFile(path)
		expect(err, "Couldn't find any cookies for account "+a.Name)
		expect(json.Unmarshal(raw, &a.Auth),
			"Unknown json in cookie file for account "+a.Name)
	}
}

// Populate client's hash table of previously downloaded books from a
// json file.
func getDownloaded(client Client, datadir string) {
	var books []Book
	path := datadir + "downloaded_books.json"
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
		client.Downloaded[b.Title] = b
	}
}

////////////////////////////////////////////////////////////////////////
//             _   _
//   __ _  ___| |_(_) ___  _ __  ___
//  / _` |/ __| __| |/ _ \| '_ \/ __|
// | (_| | (__| |_| | (_) | | | \__ \
//  \__,_|\___|\__|_|\___/|_| |_|___/
////////////////////////////////////////////////////////////////////////

func doImportCookies(client Client, account, harpath, datadir string) {
	account, err := needAccount(client, account)
	authpath := datadir + account + ".cookies.json"
	unwrap(err)
	a := client.FindAccount(account)
	raw, err := ioutil.ReadFile(harpath)
	unwrap(err)
	a.ImportCookiesFromHAR(raw)
	json, _ := json.MarshalIndent(a.Auth, "", "  ")
	unwrap(ioutil.WriteFile(authpath, json, 0644))
	fmt.Printf("Imported cookies from %s into %s\n", harpath, authpath)
}

func doConvertSingleBook(client Client, account string, aaxpath string) {
	account, err := needAccount(client, account)
	unwrap(err)
	a := client.FindAccount(account)
	var m4bpath string
	if aaxpath[len(aaxpath)-4:] == ".aax" {
		m4bpath = aaxpath[:len(aaxpath)-4] + ".m4b"
	} else {
		m4bpath = aaxpath + ".m4b"
	}
	err, ffmpegstderr := a.Convert(aaxpath, m4bpath, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ffmpegstderr)
		log.Fatalf("Failed to convert %s with bytes %s\n",
			filepath.Base(aaxpath), a.Bytes)
	}
	fmt.Printf("Made %s with %s's bytes\n", filepath.Base(m4bpath), a.Name)
}

func doScrapeLibrary(client Client, account string) {
	fmt.Println("SCRAPING LIBRARY")
	fmt.Println("INTO TEMP    ", client.TempDir)
	fmt.Println("THEN INTO DIR", client.SaveDir)
	var toscrape []Account
	if a := client.FindAccount(account); a != nil {
		toscrape = append(toscrape, *a)
		if !a.Scrape {
			log.Fatalf("Account %s has `scrape' set to false.",
				a.Name)
		}
	} else {
		toscrape = client.Accounts
	}
	for _, a := range toscrape {
		if !a.Scrape {
			continue
		}
		fmt.Println("FOR ACCOUNT  ", a.Name)
		fmt.Println("WITH BYTES   ", a.Bytes)

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
		books, err := a.ScrapeFullLibrary(ch)
		wg.Wait()

		if err != nil {
			fmt.Fprintf(os.Stderr, "BEGIN SCRAPER LOG\n")
			a.PrintScraperDebuggingInfo()
			fmt.Fprintf(os.Stderr, "END SCRAPER LOG\n")
			log.Println(err)
			fmt.Fprintf(os.Stderr, debugScraperMessage, a.Name, a.Name)
		}

		for _, b := range books {
			fmt.Println(b)
		}
	}
}
