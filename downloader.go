//
// Download a book
//

package main

import (
	"io"
	"os"
	"fmt"
	"errors"
	"regexp"
	"net/http"
)

////////////////////////////////////////////////////////////////////////
// Remove whitespace and other shell reserve characters from S
func stripstr(s string) (string) {
	r := regexp.MustCompile(`\s|\\|\(|\)|\[|\]|\*`)
	s = r.ReplaceAllString(s, "")

	return s
}

////////////////////////////////////////////////////////////////////////
// Take a pointer to a book struct, download it, and return its path
func DownloadBook(b Book) (string, error) {
	aaxname := ".audible-dl-tmp/" + stripstr(b.Title) + ".aax"
	fmt.Printf("\tDownloading %s...", aaxname)

	// Append .part to differentiate partially downloaded files
	out, err := os.Create(aaxname + ".part")
	if err != nil {
		return "", err
	}
	defer out.Close()

	resp, err := http.Get(b.DownloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("Request returned " + resp.Status)
	}

	bytes, err := io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	if bytes != resp.ContentLength {
		return "", errors.New("Failed to write file to disk")
	}

	// Rename the file now that it has been fully downloaded
	os.Rename(aaxname + ".part", aaxname)

	fmt.Printf("ok\n")
	return aaxname, nil
}
