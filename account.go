package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/html"
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
)

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

func (a *Account) Convert(in, out string, client *Client) (error, []byte) {
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
	book.DownloadURL = "https://www.audible.com/library/download?asin=" +
		book.Slug + "&codec=AAX"
	book.FileName = stripstr(book.Title)
	return book
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

// Download a single .aax file from Audible's website using the URL
// discovered by the scraper.  The file is downloaded to a .aax file
// in the temp directory, with an intermediate .part while
// downloading.  The path to the aax is returned in order to be passed
// to the converter.
func (a *Account) DownloadSingleBook(client *Client, book Book) string {
	aax := client.TempDir + book.FileName + ".aax"
	out, err := os.Create(aax + ".part")
	unwrap(err)

	jar, _ := cookiejar.New(nil)
	httpcl := &http.Client{Jar: jar}
	req, _ := http.NewRequest("GET", book.DownloadURL, nil)

	jaruri, _ := url.ParseRequestURI(book.DownloadURL)
	jar.SetCookies(jaruri, a.Auth)

	resp, err := httpcl.Do(req)
	unwrap(err)
	if resp.StatusCode != http.StatusOK {
		log.Fatal("Request returned " + resp.Status)
	}

	nbytes, err := io.Copy(out, resp.Body)
	unwrap(err)
	if nbytes != resp.ContentLength {
		log.Fatal("Failed to write file to disk")
	}

	unwrap(os.Rename(aax+".part", aax))
	return aax
}

////////////////////////////////////////////////////////////////////////
//                                             _   _ _ _ _
//  ___  ___ _ __ __ _ _ __   ___ _ __   _   _| |_(_) (_) |_ ___  ___
// / __|/ __| '__/ _` | '_ \ / _ \ '__| | | | | __| | | | __/ _ \/ __|
// \__ \ (__| | | (_| | |_) |  __/ |    | |_| | |_| | | | ||  __/\__ \
// |___/\___|_|  \__,_| .__/ \___|_|     \__,_|\__|_|_|_|\__\___||___/
//                    |_|
////////////////////////////////////////////////////////////////////////

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
