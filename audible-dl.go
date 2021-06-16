//
// audible-dl.go --- A simple tool for archiving your audible library
// git.sr.ht/~thalia/audible-dl
// Thalia Wright <vesperous@protonmail.com>
//

package main

import (
	"os"
	"fmt"
)

//
// TEMPORARY DRIVER CODE FOR LIBRARY SCRAPER
//
func main() {

	_, err := GetAllBooks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		fmt.Fprintf(os.Stderr, "I wasn't able to scrape you library :(")
		os.Exit(1)
	}
}
