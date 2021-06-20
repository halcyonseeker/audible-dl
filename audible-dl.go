//
// audible-dl.go --- A simple tool for archiving your audible library
// git.sr.ht/~thalia/audible-dl
// Thalia Wright <vesperous@protonmail.com>
//

package main

import (
	"os"
	"fmt"
	"log"
)

//
// TEMPORARY DRIVER CODE FOR LIBRARY SCRAPER
//
func main() {

	err := os.Mkdir(".audible-dl-tmp", 0755)
	if err != nil {
		if err.(*os.PathError).Err.Error() == "file exists" {
			log.Fatal("TODO: attempt to recover from error")
		}
	}

	books, err := GetAllBooks()
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		file, err := DownloadBook(books[i])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(file)
	}
}
