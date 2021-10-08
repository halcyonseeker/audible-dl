//
// audible-dl.go --- A simple tool for archiving your audible library
// sr.ht/~thalia/audible-dl
// ꙮ <ymir@ulthar.xyz>
//

package main

import (
	"io"
	"os"
	"log"
	"fmt"
	"flag"
	"errors"
	"strings"
	"os/exec"
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
	// Directory to cache downloaded files in
	Cache string
}

// Convert the aax file corresponding to the book argument into a DRM-free
// m4b file using the decryption key passed in cfg.Bytes.
func convertAAXToM4B(b *Book, cfg *ADLData) error {
	in := cfg.Cache + b.FileName + ".aax"
	out := cfg.Cache + b.FileName + ".m4b"

	cmd := exec.Command("ffmpeg",
		"-activation_bytes", cfg.Bytes,
		"-i", in,
		"-c", "copy",
		out)
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return err
	}

	// Move the now fully converted m4b file out of the temp dir and
	// remove the aax file
	if err := os.Rename(out, b.FileName + ".m4b"); err != nil {
		return err
	}
	if err := os.Remove(in); err != nil {
		return err
	}

	return nil
}

// Download the audiobook passed as an argument, saving it as an aax file
// in the cache directory
func downloadSingleBook(b *Book, cfg *ADLData) error {
	aax := cfg.Cache + b.FileName + ".aax"

	// Append .part to differentiate partially downloaded files
	out, err := os.Create(aax + ".part")
	if err != nil {
		return err
	}

	// Interestingly, the downlaod URLs are publicly accessible
	resp, err := http.Get(b.DownloadURL)
	if err != nil {
		return err
	} else {
		if resp.StatusCode != http.StatusOK {
			return errors.New("Request returned " + resp.Status)
		}
	}

	bytes, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	} else {
		if bytes != resp.ContentLength {
			return errors.New("Failed to write file to disk")
		}
	}

	// Rename the file now that it has been fully downloaded
	if err := os.Rename(aax + ".part", aax); err != nil {
		return err
	}

	return nil
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
	// might expire so we process them regardless of the its presence.

	if bytes != "" {
		cfg.Bytes = bytes
		if err := writeDataFile(&cfg); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\033[1m Set Activation Bytes to \033[m %s\n", bytes)
	}

	if harpath != "" {
		if err := importCookiesFromHAR(harpath, &cfg); err != nil {
			log.Fatal(err)
		}
		if err := writeDataFile(&cfg); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\033[1mImported Cookies From\033[m %s\n", harpath)
	}

	// Create cache dir to store partially downloaded audiobook files.
	usercache, err := os.UserCacheDir()
	if err != nil {
		log.Fatal(err)
	}
	cfg.Cache = usercache + "/audible-dl/"
	if err := os.Mkdir(cfg.Cache, 0755); err != nil {
		if !os.IsExist(err) {
			log.Fatal(err)
		}
	}

	// Scrape a list of all your audiobooks from audible's website then
	// download and convert the ones that don't appear to be present in
	// the current working directory.

	books, err := RetrieveBooksListing(&cfg)
	if err != nil {
		fmt.Printf("We received the following error while scrapping " +
			"your library:\n\n\t%s\n\nThis could mean:\n" +
			" 1. Your computer isn't connected to the internet.\n" +
			" 2. Your cached Audible cookies have expired.  If " +
			"you think that this might\n" +
			"    be the case, get a new HAR file and try " +
			"re-importing them with:\n\n" +
			"\t audible-dl -i [file.har]\n\n" +
			" 3. If the previous step fails then Audible " +
			"probably changed the structure\n" +
			"    of their website; please email a report to " +
			"~thalia/audible-dl@lists.sr.ht\n", err)
		os.Exit(1)
	}

	for i := 0; i < 3; i++ {
		b := books[i]
		if _, err := os.Stat(b.FileName + ".m4b"); err != nil {
			if os.IsNotExist(err) {
				// Download and convert the book
				fmt.Println("Downloading", b.Title)
				if err := downloadSingleBook(&b, &cfg); err != nil {
					log.Fatal(err)
				}
				if err := convertAAXToM4B(&b, &cfg); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}
