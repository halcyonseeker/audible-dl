//
// audible-dl.go --- A simple tool for archiving your audible library
// git.sr.ht/~thalia/audible-dl
// Thalia Wright <vesperous@protonmail.com>
//

package main

import (
	"os"
	"log"
	"fmt"
	"regexp"
)

// Inefficiently compute and return a list of the books we don't have downloaded
func getFarsideBooks(far []Book) []Book {
	dirents, _ := os.ReadDir(".")

	if len(dirents) == len(far) {
		return []Book{}
	}

	// THIS IS HORRIFICALLY INEFFICIENT
	for i := 0; i < len(far); i++ {
		for j := 0; j < len(dirents); j++ {
			r := regexp.MustCompile(`\.opus$`)
			name := r.ReplaceAllString(dirents[j].Name(), "")
			if name == far[i].FileName {
				far = append(far[:i], far[i+1])
			}
		}
	}
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
	if len(latest) == 0 {
		fmt.Println("\033[1mNo new books to download :)\033[m")
		os.Remove(".audible-dl-downloading")
		os.Remove(".audible-dl-converting")
		os.Exit(0)
	}

	fmt.Println("\033[1mDownloading Books:\033[m")
	for _, b := range latest {
		if err := DownloadBook(b); err != nil {
			log.Print(err)
		}
	}

	fmt.Println("\033[1mConverting Books:\033[m")
	for _, b := range latest {
		if err := CrackAAX(b.FileName); err != nil {
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
