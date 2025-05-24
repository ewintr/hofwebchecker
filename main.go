package main

import (
	"fmt"
	"os"
)

const (
	URL = "https://www.hofweb.nl/groente-aardappels/2e-klas-groentes"
)

func main() {
	fmt.Println("hofweb")
	check := NewCheck(URL)
	products, err := check.Do()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("%d producten beschikbaar\n\n", len(products))
	for _, p := range products {
		fmt.Printf("%s: %s\n", p.Name, p.URL)
	}
}
