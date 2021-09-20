package main

// Store general runtime data. For now we only have they bytes field
type Config struct {
	// You Audible activation bytes, required to decrypt book
	Bytes string
}
