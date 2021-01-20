package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	arg := os.Args[1]
	if arg != "" {
		t1 := time.Now()
		parseScore(arg)
		dur := time.Since(t1)
		fmt.Println("Created", arg+".wav", "in", dur.String())

	} else {
		log.Fatal("No score provided")
	}

}

func parseScore(name string) {
	var s Score
	err := Parse(&s, name)
	if err != nil {
		log.Fatalf("Cannot parse score file - %v", err)
	}
}
