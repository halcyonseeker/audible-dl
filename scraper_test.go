//
// go test scraper.go scraper_test.go
//

package main

import (
	"testing"
)

func prettyPrintBook(b Book, t *testing.T) {
	t.Log("\033[1m===========================================\033[m")
	t.Log("\033[1mSLUG:\033[m        ", b.Slug)
	t.Log("\033[1mTITLE:\033[m       ", b.Title)
	t.Log("\033[1mSERIES:\033[m      ", b.Series)
	t.Log("\033[1mNUMBER:\033[m      ", b.SeriesIndex)
	t.Log("\033[1mRUNTIME:\033[m     ", b.Runtime)
	t.Log("\033[1mSUMMARY:\033[m     ", b.Summary)
	t.Log("\033[1mAUTHOR(S)\033[m:   ", b.Authors)
	t.Log("\033[1mNARRATOR(S)\033[m: ", b.Narrators)
	t.Log("\033[1m-------------------------------------------\033[m")
	t.Log("\033[1mCOVER URL:\033[m   ", b.CoverURL)
	t.Log("\033[1mDOWNLOAD U\033[mRL:", b.DownloadURL)
	t.Log("\033[1mRESOURCE U\033[mRL:", b.CompanionURL)
	t.Log("\033[1m===========================================\033[m")
}

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
		prettyPrintBook(b, t)
	}
}
