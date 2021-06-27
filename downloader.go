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
	aaxname := ".audible-dl-downloading/" + b.FileName + ".aax"
	fmt.Printf("\tDownloading %s...", b.FileName)

	// Append .part to differentiate partially downloaded files
	out, err := os.Create(aaxname + ".part")
	if err != nil {
		fmt.Printf("\n")
		return err
	}
	defer out.Close()

	resp, err := http.Get(b.DownloadURL)
	if err != nil {
		fmt.Printf("\033[32mfailed\033[m\n")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("\033[32mfailed\033[m\n")
		return errors.New("Request returned " + resp.Status)
	}

	bytes, err := io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("\033[32mfailed\033[m\n")
		return err
	}

	if bytes != resp.ContentLength {
		fmt.Printf("\033[32mfailed\033[m\n")
		return errors.New("Failed to write file to disk")
	}

	// Rename the file now that it has been fully downloaded
	os.Rename(aaxname + ".part", aaxname)

	fmt.Printf("ok\n")
	return nil
}
