//
// Crack an .aax file and clean up the .opus' metadata
//

package main

import (
	"os"
	"fmt"
	"os/exec"
)

////////////////////////////////////////////////////////////////////////
// Take a book struct with a corresponding aax file and convert it to opus
func CrackAAX(filename string) error {
	infile  := "./.audible-dl-downloading/" + filename + ".aax"
	outfile := "./.audible-dl-converting/" + filename + ".opus"
	bytes := "deadbeef"

	fmt.Printf("\tConverting %s...", filename)
	cmd := exec.Command("ffmpeg",
		"-activation_bytes", bytes, // Key to decrypt .aax file
		"-i", infile,				// Specify the input
		"-vn",						// We'll encode the cover image later
		"-c:a", "libopus",			// Use the libopus encoder
		outfile)
	cmd.Stdout = nil                // Send output to /dev/null

	// We'll return when the process completes
	err := cmd.Run()
	if err != nil {
		fmt.Printf("\033[31mfailed\033[m\n")
		return err
	}

	// Move the opus out of the temp dir and remove the aax file. This means
	// that if the conversion fails or is interrupted we won't get broken
	// audiobooks mixed in with the good ones.
	os.Rename(outfile, filename + ".opus")
	os.Remove(infile)

	fmt.Printf("ok\n")
	return nil
}
