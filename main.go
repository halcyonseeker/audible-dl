//
// audible-dl.go --- A simple tool for archiving your audible library
// sr.ht/~thalia/audible-dl
// ꙮ <ymir@ulthar.xyz>
//

package main

import (
	"os"
	"log"
	"fmt"
	"flag"
	"strings"
	"net/http"
	"io/ioutil"
	"encoding/json"
)

// An instance of this holds configuration data read from command-line
// arguments and the ./.audible-dl.json file.
type ADLData struct {
	// Activation bytes, required to decrypt the audiobooks
	Bytes string
	// Cookies, so that we can scrape your personal library
	Cookies []*http.Cookie
}

// Owing to the general complexity of Amazon's login process, it's a lot
// easier to just import the cookies we need from a HAR file than it is to
// fetch the information ourselves.  This file is created by sending a GET
// request to audible.com/library/titles while signed in.  The HAR file
// should be passed with the -i flag on first run; subsequent runs quietly
// read cookies from ./.audible-dl.json.
func importCookiesFromHAR(path string, cfg *ADLData) error {
	var har map[string]interface{}

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(raw, &har); err != nil {
		return err
	}

	cookies := har["log"].
		(map[string]interface{})["entries"].
		([]interface{})[0].
		(map[string]interface{})["request"].
		(map[string]interface{})["cookies"].
		([]interface{})
	for _, c := range cookies {
		value := c.(map[string]interface{})["value"].(string)
		// The values of some non-essential cookies contain a double
		// quote character which net/http really doesn't like
		if strings.Contains(value, "\"") {
			continue
		}
		cfg.Cookies = append(cfg.Cookies, &http.Cookie{
			Name: c.(map[string]interface{})["name"].(string),
			Value: value,
		})
	}

	return nil
}

// Read the contents of ./.audible-dl.json into an ADLData struct.  This
// file is used to persistently store essential program information like
// authentication cookies and activation bytes.
func readDataFile(cfg *ADLData) error {
	raw, err := ioutil.ReadFile(".audible-dl.json")
	if err != nil {
		return err
	}

	if err = json.Unmarshal(raw, &cfg); err != nil {
		return err
	}

	return nil
}

// Write an ADLData struct to ./.audible-dl.json.  This should probably
// only be done when the user first runs this program with the -i and -b
// flags. Subsequent runs should just read the required data from the file.
func writeDataFile(cfg *ADLData) error {
	json, _ := json.MarshalIndent(cfg, "", "  ")
	if err := ioutil.WriteFile(".audible-dl.json", json, 0644); err != nil {
		return err
	}

	return nil
}

func main() {
	var cfg ADLData
	var bytes, harpath string

	flag.StringVar(&bytes, "b", "", "Your Audible activation bytes.")
	flag.StringVar(&harpath, "i", "", "Import a HAR file.")
	flag.Parse()

	// Read required information from the data file.  If it isn't present,
	// tell the user how to create it.
	if err := readDataFile(&cfg); err != nil {
		if os.IsNotExist(err) && (bytes == "" || harpath == "") {
			fmt.Printf("Data file is not present.  Please cd to " +
				"your audibooks directory or\n" +
				"initialise an archive here by running:\n\n" +
				"\t audible-dl -b [bytes] -i [file.har]\n\n" +
				"For more information or to report a bug, " +
				"see the project website:\n" +
				"\thttps://sr.ht/~thalia/audible-dl\n")
			os.Exit(0)
		} else if !os.IsNotExist(err) {
			log.Fatal(err)
		}
	}

	// Handle command-line arguments used to build the data file.  Cookies
	// might expire so we process them regaurdless of the its presence.

	if bytes != "" {
		cfg.Bytes = bytes
		if err := writeDataFile(&cfg); err != nil {
			log.Fatal(err)
		}
	}

	if harpath != "" {
		if err := importCookiesFromHAR(harpath, &cfg); err != nil {
			log.Fatal(err)
		}
		if err := writeDataFile(&cfg); err != nil {
			log.Fatal(err)
		}
	}

	// Create data files to store partially downloaded and converted
	// audiobook files.

	if err := os.Mkdir(".audible-dl-downloading", 0664); err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
	}

	if err := os.Mkdir(".audible-dl-converting", 0644); err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
	}

	// Scrape a list of all your audiobooks from audible's website then
	// download and convert the ones that don't appear to be present in
	// the current working directory.

	books, err := RetrieveBooksListing(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	for _, b := range books {
		if _, err := os.Stat(b.FileName + ".m4b"); err != nil {
			if os.IsNotExist(err) {
				// Download and convert the book
				fmt.Println("I'm gonna get", b.Title)
			}
		}
	}

	// By now we should have downloaded and converted all the audiobooks
	// not present in the current directory, so we can safely remove the
	// cache directories.

	if err := os.Remove(".audible-dl-downloading"); err != nil {
		log.Fatal(err)
	}

	if err := os.Remove(".audible-dl-converting"); err != nil {
		log.Fatal(err)
	}
}
