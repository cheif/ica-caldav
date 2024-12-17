package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)


func main() {
    sessionId := os.Getenv("SESSION_ID")
    ica := NewICA(sessionId)
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

type ICA struct {
    sessionId string
}

func NewICA(sessionId string) ICA {
    return ICA { sessionId }
}

type ShoppingList struct {
    Id string `json:"id"`
    Name string `json:"name"`
}

func (ica *ICA) GetShoppingLists() ([]ShoppingList, error) {
    data, err := ica.get("shopping-list/v1/api/list/all")
    if err != nil {
        return nil, err
    }
    var lists []ShoppingList
    err = json.Unmarshal(data, &lists)
    if err != nil {
        return nil, err
    }
    return lists, nil
}

type Suggestion struct {
    Name string `json:"name"`
    Id int `json:"id"`
}

type searchResponse struct {
    Documents []Suggestion `json:"documents"`
}

func (ica *ICA) SearchItem(name string) ([]Suggestion, error) {
    path := fmt.Sprintf("shoppinglistarticlesearch/v1/search?query=%v", name)
    data, err := ica.get(path)
    if err != nil {
        return nil, err
    }
    var response searchResponse
    err = json.Unmarshal(data, &response)
    return response.Documents, err
}

type ShoppingListItem struct {
    Name string `json:"text"`
    Article *Suggestion `json:"article"`
}

func (ica *ICA) AddItem(list ShoppingList, suggestion Suggestion) error {
    item := ShoppingListItem {
        Name: suggestion.Name,
        Article: &suggestion,
    }
    return ica.addItem(list, item)
}

func (ica *ICA) addItem(list ShoppingList, item ShoppingListItem) error {
    path := fmt.Sprintf("shopping-list/v1/api/list/%v/row", list.Id)
    data, err := json.Marshal(item)
    if err != nil {
        return err
    }
    _, err = ica.post(path, data)
    return err
}

func (ica *ICA) get(path string) ([]byte, error) {
    url := fmt.Sprintf("https://apimgw-pub.ica.se/sverige/digx/%v", path)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    return ica.do(req)
}

func (ica *ICA) post(path string, data []byte) ([]byte, error) {
    url := fmt.Sprintf("https://apimgw-pub.ica.se/sverige/digx/%v", path)
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
    req.Header.Set("Content-Type", "application/json")
    if err != nil {
        return nil, err
    }
    return ica.do(req)
}

func (ica *ICA) do(req *http.Request) ([]byte, error) {
    client := &http.Client{}
    token, err := ica.getToken()
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", token))
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    return io.ReadAll(resp.Body)
}

func (ica *ICA) getToken() (string, error) {
    client := &http.Client{}
    req, err := http.NewRequest("GET", "https://www.ica.se/api/user/information", nil)
    if err != nil {
        return "", err
    }
    req.AddCookie(&http.Cookie{Name: "thSessionId", Value: ica.sessionId})
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    var tokenResponse tokenResponse
    err = json.Unmarshal(data, &tokenResponse)
    return tokenResponse.AccessToken, err
}

type tokenResponse struct {
    AccessToken string `json:"accessToken"`
}
