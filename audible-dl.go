//
// audible-dl.go --- A simple tool for archiving your audible library
// git.sr.ht/~thalia/audible-dl
// Thalia Wright <vesperous@protonmail.com>
//

package main

import (
	"os"
	"log"
)

// Later we'll generate a list of the ones we don't have
func getFarsideBooks(far []Book) []Book {
	return far
}

//
// TEMPORARY DRIVER CODE FOR LIBRARY SCRAPER
//
func main() {

	err := os.Mkdir(".audible-dl-downloading", 0755)
	if err != nil {
		if err.(*os.PathError).Err.Error() == "file exists" {
			log.Fatal("TODO: attempt to recover from error")
		}
	}

	err := os.Mkdir(".audible-dl-converting", 0755)
	if err != nil {
		if err.(*os.PathError).Err.Error() == "file exists" {
			log.Fatal("TODO: attempt to recover from error")
		}
	}

	books, err := GetAllBooks()
	if err != nil {
		log.Fatal(err)
	}

	latest := getFarsideBooks(books)

	for i := 0; i < 1; i++ {
		err := DownloadBook(latest[i])
		if err != nil {
			log.Fatal(err)
		}
	}

	for i := 0; i < 1; i++ {
		err := CrackAAX(latest[i])
		if err != nil {
			log.Fatal(err)
		}
	}
}
