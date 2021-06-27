//
// audible-dl.go --- A simple tool for archiving your audible library
// git.sr.ht/~thalia/audible-dl
// ꙮ <ymir@ulthar.xyz>
//

package main

import (
	"os"
	"log"
	"fmt"
)

// Later we'll generate a list of the ones we don't have
func getFarsideBooks(far []Book) []Book {
	return far
}

//
// TEMPORARY DRIVER CODE FOR LIBRARY SCRAPER
//
func main() {

	if err := os.Mkdir(".audible-dl-downloading", 0755); err != nil {
		if err.(*os.PathError).Err.Error() == "file exists" {
			log.Fatal("TODO: attempt to recover from error")
		}
	}

	if err := os.Mkdir(".audible-dl-converting", 0755); err != nil {
		if err.(*os.PathError).Err.Error() == "file exists" {
			log.Fatal("TODO: attempt to recover from error")
		}
	}

	fmt.Println("\033[1mScraping Library:\033[m")
	books, err := GetAllBooks()
	if err != nil {
		log.Fatal(err)
	}

	latest := getFarsideBooks(books)

	fmt.Println("\033[1mDownloading Books:\033[m")
	for i := 0; i < 1; i++ {
		if err := DownloadBook(latest[i]); err != nil {
			log.Print(err)
		}
	}

	fmt.Println("\033[1mConverting Books:\033[m")
	for i := 0; i < 1; i++ {
		if err := CrackAAX(latest[i].FileName); err != nil {
			log.Print(err)
		}
	}

	if err = os.Remove(".audible-dl-downloading"); err != nil {
		log.Print("Some download operations failed.")
	}

	if err = os.Remove(".audible-dl-converting"); err != nil {
		log.Print("Some conversion operations failed.")
	}
}
