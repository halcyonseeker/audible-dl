//
// audible-dl.go --- A simple tool for archiving your audible library
// git.sr.ht/~thalia/audible-dl
// ꙮ <ymir@ulthar.xyz>
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

	books, err := GetAllBooks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		fmt.Fprintf(os.Stderr, "I wasn't able to scrape you library :(")
		os.Exit(1)
	}

	for _, b := range books {
		fmt.Println("==================================================")
		fmt.Println("SLUG:        ", b.Slug)
		fmt.Println("TITLE:       ", b.Title)
		fmt.Println("SERIES:      ", b.Series)
		fmt.Println("NUMBER:      ", b.SeriesIndex)
		fmt.Println("RUNTIME:     ", b.Runtime)
		fmt.Println("SUMMARY:     ", b.Summary)
		fmt.Println("AUTHOR(S):   ", b.Authors)
		fmt.Println("NARRATOR(S): ", b.Narrators)
		fmt.Println("--------------------------------------------------")
		fmt.Println("COVER URL:   ", b.CoverURL)
		fmt.Println("DOWNLOAD URL:", b.DownloadURL)
		fmt.Println("RESOURCE URL:", b.CompanionURL)
		fmt.Println("==================================================")
	}
}
