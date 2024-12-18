package main

import (
	"fmt"
	"os"

    "ica-caldav/ica"
)


func main() {
    sessionId := os.Getenv("SESSION_ID")
    ica := ica.New(sessionId)
    lists, err := ica.GetShoppingLists()
    if err != nil {
        fmt.Printf("Error: %v", err)
    }
    fmt.Println(lists)
    items, err := ica.SearchItem("Vaniljglass")
    if err != nil {
        fmt.Printf("Error: %v", err)
    }
    fmt.Println(items)
    err = ica.AddItem(lists[0], items[0])
    if err != nil {
        fmt.Printf("Error: %v", err)
    }
}
