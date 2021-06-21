//
// Download a book
//

package main

import (
	"io"
	"os"
	"fmt"
	"errors"
	"net/http"
)

////////////////////////////////////////////////////////////////////////
// Take a pointer to a book struct, download it, and return its path
func DownloadBook(b Book) error {
	aaxname := ".audible-dl-tmp/" + b.FileName + ".aax"
	fmt.Printf("\tDownloading %s...", aaxname)

	// Append .part to differentiate partially downloaded files
	out, err := os.Create(aaxname + ".part")
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(b.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("Request returned " + resp.Status)
	}

	bytes, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	if bytes != resp.ContentLength {
		return errors.New("Failed to write file to disk")
	}

	// Rename the file now that it has been fully downloaded
	os.Rename(aaxname + ".part", aaxname)

	fmt.Printf("ok\n")
	return nil
}
