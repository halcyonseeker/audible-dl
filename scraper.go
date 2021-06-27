//
// Scrape an audible library
//

package main

import (
	"os"
	"fmt"
	"bytes"
	"errors"
	"regexp"
	"strings"
	"strconv"
	"net/url"
	"net/http"
	"net/http/cookiejar"
	"encoding/json"
	"io/ioutil"
	"golang.org/x/net/html"
)

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

////////////////////////////////////////////////////////////////////////
// Pretty-print the fields of a book structure
func debugPrintBook(b Book) {
	fmt.Println("\033[1m================================================\033[m")
	fmt.Println("\033[1mSLUG:\033[m        ", b.Slug)
	fmt.Println("\033[1mTITLE:\033[m       ", b.Title)
	fmt.Println("\033[1mSERIES:\033[m      ", b.Series)
	fmt.Println("\033[1mNUMBER:\033[m      ", b.SeriesIndex)
	fmt.Println("\033[1mRUNTIME:\033[m     ", b.Runtime)
	fmt.Println("\033[1mSUMMARY:\033[m     ", b.Summary)
	fmt.Println("\033[1mAUTHOR(S)\033[m:   ", b.Authors)
	fmt.Println("\033[1mNARRATOR(S)\033[m: ", b.Narrators)
	fmt.Println("\033[1m------------------------------------------------\033[m")
	fmt.Println("\033[1mCOVER URL:\033[m   ", b.CoverURL)
	fmt.Println("\033[1mDOWNLOAD U\033[mRL:", b.DownloadURL)
	fmt.Println("\033[1mRESOURCE U\033[mRL:", b.CompanionURL)
	fmt.Println("\033[1m================================================\033[m")
}

////////////////////////////////////////////////////////////////////////
// Return cached cookies to trick Audible into thinking we're a browser session
func getSessionCookies() ([]*http.Cookie, error) {
	var cookies []*http.Cookie

	// TODO: If the cookies are old or not present, get new ones.
	// For now we'll just assume the cookie file is present and
	// contains valid cookies retrieved manually with inspect element.
	// IIrc Audible.com's session cookies should last about 2 months.
	raw, err := ioutil.ReadFile(".audible-dl-cookies.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return nil, errors.New("Failed to open cookie file")
	}

	jerr := json.Unmarshal(raw, &cookies)
	if jerr != nil {
		fmt.Fprintf(os.Stderr, "%s\n", jerr)
		return nil, errors.New("Cookie file contains invalid json")
	}

	return cookies, nil
}

////////////////////////////////////////////////////////////////////////
// Download a HTMl page in the user's library
func getLibraryPage(page int) ([]byte, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{ Jar: jar, }
	uri := "https://www.audible.com/library/titles?page=" + strconv.Itoa(page)
	req, _ := http.NewRequest("GET", uri, nil)

	cookies, _ := getSessionCookies()
	jaruri, _  := url.ParseRequestURI(uri)
	jar.SetCookies(jaruri, cookies)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// fmt.Println("+=============================== Request Cookies")
	// for _, c := range cookies {
	// 	fmt.Printf("| %s: %s\n", c.Name, c.Value)
	// }
	// fmt.Println("+=============================== Response Cookies")
	// for _, c := range resp.Cookies() {
	// 	fmt.Printf("| %s: %s\n", c.Name, c.Value)
	// }
	// fmt.Println("+=============================== Actual Output")

	if resp.StatusCode != 200 {
		return nil, errors.New("getLibraryPage: " + resp.Status)
	}

	html, _ := ioutil.ReadAll(resp.Body)
	return html, nil
}

////////////////////////////////////////////////////////////////////////
// Remove whitespace and other shell reserve characters from S
func stripstr(s string) (string) {
	r := regexp.MustCompile(`\s|\\|\(|\)|\[|\]|\*`)
	s = r.ReplaceAllString(s, "")

	return s
}

////////////////////////////////////////////////////////////////////////
// Remove extra whitespace and newlines from a string
func cleanstr(s string) (string) {
	r := regexp.MustCompile(`\s+`)
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\n")
	s = r.ReplaceAllString(s, " ")

	return s
}

////////////////////////////////////////////////////////////////////////
// Get the class attribute of html tag T
func class(t html.Token) (string) {
	for _, a := range t.Attr {
		if a.Key == "class" {
			return cleanstr(a.Val)
		}
	}
	return ""
}

////////////////////////////////////////////////////////////////////////
// Get the href attribute of html tag T
func href(t html.Token) (string) {
	for _, a := range t.Attr {
		if a.Key == "href" {
			return cleanstr(a.Val)
		}
	}
	return ""
}

////////////////////////////////////////////////////////////////////////
// Get the id attribute of html tag T
func id(t html.Token) (string) {
	for _, a := range t.Attr {
		if a.Key == "id" {
			return cleanstr(a.Val)
		}
	}
	return ""
}

////////////////////////////////////////////////////////////////////////
// Get the aria-label (how is this different from id??) attriute of hmtl tag T
func aria_label(t html.Token) (string) {
	for _, a := range t.Attr {
		if a.Key == "aria-label" {
			return cleanstr(a.Val)
		}
	}
	return ""
}

////////////////////////////////////////////////////////////////////////
// Get the book's author and narrator (translator?)
func xPeople(dom *html.Tokenizer) ([]string) {
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

////////////////////////////////////////////////////////////////////////
// Get the book's summary
func xSummary(dom *html.Tokenizer, tt html.TokenType, tok html.Token) (string) {
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

////////////////////////////////////////////////////////////////////////
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

////////////////////////////////////////////////////////////////////////
// Fill in the structure for a single book
func xSingleBook(dom *html.Tokenizer, tt html.TokenType, tok html.Token) (Book) {
	var book Book

	// First we'll extract the book's slug from its div's id
	slug := id(tok)
	book.Slug = slug[len(slug) - 10:]

	for {
		tt = dom.Next()
		tok = dom.Token()

		if strings.Contains(class(tok), "bc-image-inset-border")  {
			////////// COVER IMAGE
			for _, a := range tok.Attr {
				if a.Key == "src" {
					book.CoverURL = cleanstr(a.Val)
				}
			}

		} else if class(tok) == "bc-text bc-size-headline3" {
			////////// TITLE
			tt = dom.Next()
			tok = dom.Token()
			book.Title = cleanstr(tok.Data)
			continue

		} else if strings.Contains(class(tok), "authorLabel") {
			////////// AUTHORS
			book.Authors = xPeople(dom)
			continue

		} else if strings.Contains(class(tok), "narratorLabel") {
			////////// NARRATORS
			book.Narrators = xPeople(dom)
			continue

		} else if strings.Contains(class(tok), "merchandisingSummary") {
			////////// SUMMARY
			book.Summary = xSummary(dom, tt, tok)
			continue

		} else if id(tok) == "time-remaining-display-" + book.Slug {
			////////// RUNTIME
			for !(tt == html.EndTagToken && tok.Data == "span") {
				tt = dom.Next()
				tok = dom.Token()
				if tt == html.TextToken {
					book.Runtime = cleanstr(tok.Data)
				}
			}
			continue

		} else if strings.Contains(href(tok), "/series/") {
			////////// SERIES and SERIESINDEX
			book.Series, book.SeriesIndex = xSeries(dom, tt, tok)
			continue

		} else if aria_label(tok) != "DownloadFull" && strings.Contains(href(tok), "cds.audible.com/download") {
			////////// DOWNLOAD URL
			book.DownloadURL = href(tok)
			continue

		} else if href(tok) == "/companion-file/" + book.Slug {
			////////// DOWNLOAD URL
			book.CompanionURL = "https://audible.com" + cleanstr(href(tok))
			continue
		}

		// We've arrived at the next book
		if strings.Contains(class(tok), "library-item-divider") ||
			id(tok) == "adbl-library-content-toast-messaging"  {
			break
		}
	}

	book.FileName = stripstr(book.Title)

	return book
}

////////////////////////////////////////////////////////////////////////
// Get a slice of structs containing info about all the books in my library
func GetAllBooks() ([]Book, error) {
	var books []Book

	// audible.com/library/titles?page=N doesn't return a 404 when
	// we access pages that don't exist, so we'll store the slug of
	// the first books of the current and previous pages in these
	// respective variables and check against the current book's slug.
	var firstincurrpage string = ""
	var firstinprevpage string = ""

	for i := 1; ; i++ {
		fmt.Printf("\tPage %d...", i)
		raw, err := getLibraryPage(i)
		if err != nil {
			fmt.Printf("\033[31mfailed\033[m\n")
			return nil, err
		}

		dom := html.NewTokenizer(bytes.NewReader(raw))

		for {
			tt := dom.Next()
			if tt == html.StartTagToken {
				tok := dom.Token()

				// If we find a book, extract it
				if class(tok) == "adbl-library-content-row" {
					book := xSingleBook(dom, tt, tok)
					if book.Slug == firstinprevpage {
						// We've reached a duplicate page
						fmt.Printf("ok\n")
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

				// If it looks like we're getting to the end, break
				if id(tok) == "center-6" {
					break
				}

			} else if tt == html.ErrorToken { // EOF
				break 
			}
		}
		// We're fetching the next page, so we cycle these out
		firstinprevpage = firstincurrpage
		firstincurrpage = ""
		fmt.Printf("ok\n")

		if len(books) == 0 {
			fmt.Printf("\033[31mfailed\033[m\n")
			return nil, errors.New("I couldn't find any books in the HTML :(")
		}
	}
}
