package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

////////////////////////////////////////////////////////////////////////
//       _ _            _           _     _           _
//   ___| (_) ___ _ __ | |_    ___ | |__ (_) ___  ___| |_
//  / __| | |/ _ \ '_ \| __|  / _ \| '_ \| |/ _ \/ __| __|
// | (__| | |  __/ | | | |_  | (_) | |_) | |  __/ (__| |_
//  \___|_|_|\___|_| |_|\__|  \___/|_.__// |\___|\___|\__|
//                                     |__/
////////////////////////////////////////////////////////////////////////

type Client struct {
	SaveDir  string
	TempDir  string
	Accounts []Account
}

func (c *Client) Validate() {
	if c.SaveDir == "" {
		log.Fatal("savdir not specified on config file")
	}
	if c.TempDir == "" {
		panic("Failed to infer TempDir!")
	}
	if len(c.Accounts) == 0 {
		log.Fatal("Couldn't find any accounts in config file.")
	}
	for _, a := range c.Accounts {
		if a.Name == "" {
			log.Fatal("Account name not specified in config file.")
		}
		if a.Bytes == "" {
			log.Fatal("Activation bytes not present for account " +
				a.Name)
		}
		// It's okay not to have cookies
	}
}

func (c *Client) FindAccount(name string) *Account {
	for _, a := range c.Accounts {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////
//                                  _           _     _           _
//   __ _  ___ ___ ___  _   _ _ __ | |_    ___ | |__ (_) ___  ___| |_
//  / _` |/ __/ __/ _ \| | | | '_ \| __|  / _ \| '_ \| |/ _ \/ __| __|
// | (_| | (_| (_| (_) | |_| | | | | |_  | (_) | |_) | |  __/ (__| |_
//  \__,_|\___\___\___/ \__,_|_| |_|\__|  \___/|_.__// |\___|\___|\__|
//                                                 |__/
////////////////////////////////////////////////////////////////////////

type Account struct {
	Name   string
	Bytes  string
	Auth   []*http.Cookie
	Scrape bool
}

func (a *Account) ImportCookiesFromHAR(raw []byte) {
	var har map[string]interface{}

	unwrap(json.Unmarshal(raw, &har))

	cookies := har["log"].(map[string]interface{})["entries"].([]interface{})[0].(map[string]interface{})["request"].(map[string]interface{})["cookies"].([]interface{})

	for _, c := range cookies {
		value := c.(map[string]interface{})["value"].(string)
		// The values of some non-essential cookies contain a double
		// quote character which net/http really doesn't like
		if strings.Contains(value, "\"") {
			continue
		}
		a.Auth = append(a.Auth, &http.Cookie{
			Name:  c.(map[string]interface{})["name"].(string),
			Value: value,
		})
	}
}

func (a *Account) Convert(in, out string, client Client) (error, []byte) {
	tmp := client.TempDir + filepath.Base(out)
	cmd := exec.Command("ffmpeg",
		"-activation_bytes", a.Bytes,
		"-i", in,
		"-c", "copy",
		tmp)
	cmd.Stdout = nil
	stderr, _ := cmd.StderrPipe()
	err := cmd.Start()
	slurp, _ := io.ReadAll(stderr)
	if err != nil {
		return err, slurp
	}
	if err = cmd.Wait(); err != nil {
		return err, slurp
	}
	if err = os.Rename(tmp, out); err != nil {
		return err, nil
	}
	return nil, nil
}

////////////////////////////////////////////////////////////////////////
//             _                          _       _
//   ___ _ __ | |_ _ __ _   _ _ __   ___ (_)_ __ | |_
//  / _ \ '_ \| __| '__| | | | '_ \ / _ \| | '_ \| __|
// |  __/ | | | |_| |  | |_| | |_) | (_) | | | | | |_
//  \___|_| |_|\__|_|   \__, | .__/ \___/|_|_| |_|\__|
//                      |___/|_|
////////////////////////////////////////////////////////////////////////

func main() {
	account, harpath, aaxpath := getArgs()
	cfgfile, authdir, tempdir, savedir := getPaths()
	client := getData(cfgfile, authdir, tempdir, savedir)
	client.Validate()

	log.Println("ACCOUNT", account)
	log.Println("HARPATH", harpath)
	log.Println("AAXPATH", aaxpath)
	log.Println("")
	log.Println("cfgfile", cfgfile)
	log.Println("authdir", authdir)
	log.Println("tempdir", tempdir)
	log.Println("savedir", savedir)
	log.Println("")
	log.Println(client)
	log.Println("")

	if harpath != "" {
		doImportCookies(client, account, harpath, authdir)
		os.Exit(0)
	}

	if aaxpath != "" {
		doConvertSingleBook(client, account, aaxpath)
		os.Exit(0)
	}

	getCookies(client, authdir)
	doScrapeLibrary(client, account)
}

////////////////////////////////////////////////////////////////////////
//                   _ _ _            _
//   __ _ _   ___  _(_) (_) __ _ _ __(_) ___  ___
//  / _` | | | \ \/ / | | |/ _` | '__| |/ _ \/ __|
// | (_| | |_| |>  <| | | | (_| | |  | |  __/\__ \
//  \__,_|\__,_/_/\_\_|_|_|\__,_|_|  |_|\___||___/
////////////////////////////////////////////////////////////////////////

const helpMessage string = `Usage: audible-dl [-h] [-a ACC] [-i HAR] [-s AAX]

  Scrape your Audible library or convert an AAX file to m4b.
  See audible-dl(1) for more information.

Options:
  -h, --help         Print this message and exit.
  -a, --account NAME Specify an account for the operation.
  -i, --import  HAR  Import login cookies from HAR.
  -s, --single  AAX  Convert the single AAX file specified in AAX.
`

func unwrap(err interface{}) {
	if err != nil {
		log.Fatal(err)
	}
}

func expect(err interface{}, why string) {
	if err != nil {
		log.Print(why)
		log.Print(err)
		fmt.Fprintf(os.Stderr, helpMessage)
		os.Exit(1)
	}
}

func needAccount(client Client, account string) (string, error) {
	if len(client.Accounts) != 1 && account == "" {
		return "", errors.New(
			"You have multiple accounts set up, please specify one")
	} else if account == "" {
		return client.Accounts[0].Name, nil
	} else if len(client.Accounts) > 1 && account != "" {
		if client.FindAccount(account) == nil {
			return "", errors.New(
				"Account " + account + " doesn't exist")
		}
	}
	return account, nil
}

////////////////////////////////////////////////////////////////////////
//                  _                                      _
//   ___ _ ____   _(_)_ __ ___  _ __  _ __ ___   ___ _ __ | |_
//  / _ \ '_ \ \ / / | '__/ _ \| '_ \| '_ ` _ \ / _ \ '_ \| __|
// |  __/ | | \ V /| | | | (_) | | | | | | | | |  __/ | | | |_
//  \___|_| |_|\_/ |_|_|  \___/|_| |_|_| |_| |_|\___|_| |_|\__|
////////////////////////////////////////////////////////////////////////

func getPaths() (string, string, string, string) {
	var cfgfile, authdir, tempdir, savedir string
	root := os.Getenv("AUDIBLE_DL_ROOT")
	if root != "" {
		cfgfile = root + "/.audible-dl/config.yml"
		authdir = root + "/.audible-dl/auth/"
		tempdir = root + "/.audible-dl/temp/"
		savedir = root + "/"
	} else {
		conf, err := os.UserConfigDir()
		unwrap(err)
		cach, err := os.UserCacheDir()
		unwrap(err)
		cfgfile = conf + "/audible-dl/config.yml"
		authdir = cach + "/audible-dl/auth/"
		tempdir = cach + "/audible-dl/temp/"
		savedir = "" // Read later from config file
	}
	return cfgfile, authdir, tempdir, savedir
}

func getArgs() (string, string, string) {
	var a, h, s string
	// FIXME: prevent duplicate flags
	flag.StringVar(&a, "a", "", "")
	flag.StringVar(&h, "i", "", "")
	flag.StringVar(&s, "s", "", "")
	flag.StringVar(&a, "account", "", "")
	flag.StringVar(&h, "import", "", "")
	flag.StringVar(&s, "single", "", "")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, helpMessage)
	}
	flag.Parse()
	return a, h, s
}

func getData(cfgfile, authdir, tempdir, savedir string) Client {
	var client Client
	raw, err := os.ReadFile(cfgfile)
	expect(err, "Please create the config file with at least one account")
	expect(yaml.Unmarshal(raw, &client), "Bad yaml in config file")
	client.TempDir = tempdir
	if os.Getenv("AUDIBLE_DL_ROOT") != "" {
		client.SaveDir = savedir
	}
	return client
}

func getCookies(client Client, authdir string) {
	for _, a := range client.Accounts {
		if !a.Scrape {
			continue
		}
		path := authdir + a.Name + ".cookies.json"
		raw, err := os.ReadFile(path)
		expect(err, "Couldn't find any cookies for account "+a.Name)
		expect(json.Unmarshal(raw, &a.Auth),
			"Unknown json in cookie file for account "+a.Name)
	}
}

////////////////////////////////////////////////////////////////////////
//             _   _
//   __ _  ___| |_(_) ___  _ __  ___
//  / _` |/ __| __| |/ _ \| '_ \/ __|
// | (_| | (__| |_| | (_) | | | \__ \
//  \__,_|\___|\__|_|\___/|_| |_|___/
////////////////////////////////////////////////////////////////////////

func doImportCookies(client Client, account, harpath, authdir string) {
	account, err := needAccount(client, account)
	authpath := authdir + account + ".cookies.json"
	unwrap(err)
	a := client.FindAccount(account)
	raw, err := ioutil.ReadFile(harpath)
	unwrap(err)
	a.ImportCookiesFromHAR(raw)
	json, _ := json.MarshalIndent(a.Auth, "", "  ")
	unwrap(ioutil.WriteFile(authpath, json, 0644))
	fmt.Printf("Imported cookies from %s into %s\n", harpath, authpath)
}

func doConvertSingleBook(client Client, account string, aaxpath string) {
	account, err := needAccount(client, account)
	unwrap(err)
	a := client.FindAccount(account)
	var m4bpath string
	if aaxpath[len(aaxpath)-4:] == ".aax" {
		m4bpath = aaxpath[:len(aaxpath)-4] + ".m4b"
	} else {
		m4bpath = aaxpath + ".m4b"
	}
	err, ffmpegstderr := a.Convert(aaxpath, m4bpath, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ffmpegstderr)
		log.Fatalf("Failed to convert %s with bytes %s\n",
			filepath.Base(aaxpath), a.Bytes)
	}
	fmt.Printf("Made %s with %s's bytes\n", filepath.Base(m4bpath), a.Name)
}

func doScrapeLibrary(client Client, account string) {
	fmt.Println("SCRAPING LIBRARY")
	fmt.Println("INTO TEMP    ", client.TempDir)
	fmt.Println("THEN INTO DIR", client.SaveDir)
	var toscrape []Account
	if a := client.FindAccount(account); a != nil {
		toscrape = append(toscrape, *a)
		if !a.Scrape {
			log.Fatalf("Account %s has `scrape' set to false.",
				a.Name)
		}
	} else {
		toscrape = client.Accounts
	}
	for _, a := range toscrape {
		if !a.Scrape {
			continue
		}
		fmt.Println("FOR ACCOUNT  ", a.Name)
		fmt.Println("WITH BYTES   ", a.Bytes)
	}
}
