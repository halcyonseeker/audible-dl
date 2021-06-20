//
// go test scraper.go scraper_test.go
//

package main

import (
	"testing"
)

////////////////////////////////////////////////////////////////////////
// Make sure all the books have the required fields
func TestBooksIntegrity(t *testing.T) {
	books, err := GetAllBooks()

	if err != nil {
		t.Errorf("Expected a slice of books, received error: %v", err)
	}

	for _, b := range books {
		if b.DownloadURL == "" {
			t.Errorf("Missing download URL for %s", b.Title)
		}
	}
}
