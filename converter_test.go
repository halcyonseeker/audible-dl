//
// go test converter.go converter_test.go
//

package main

import (
	"os"
	"regexp"
	"testing"
	"io/ioutil"
)

func TestAAXConverter(t *testing.T) {
	aaxes, derr := ioutil.ReadDir(".audible-dl-downloading")
	_, serr := os.Stat(".audible-dl-converting")
	if derr != nil || serr != nil || len(aaxes) == 0 {
		t.Fatalf("This test presupposes the presence .audible-dl-converting/ and \nof one or more .aax files in .audible-dl-downloading/\n")
	}

	for _, a := range aaxes {
		r := regexp.MustCompile(`\.aax$`)
		name := r.ReplaceAllString(a.Name(), "")
		t.Logf("Converting %s\n", name)
		err := CrackAAX(name)
		if err != nil {
			t.Fatalf("%s\n", err)
		}
	}
}
