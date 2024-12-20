package ica

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mitchellh/hashstructure/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type SessionProvider interface {
	GetSession() (*ICA, error)
}

type ICA struct {
	sessionId string
}

func New(sessionId string) ICA {
	return ICA{sessionId}
}

func (ica *ICA) setSessionId(sessionId string) {
	ica.sessionId = sessionId
}

type ShoppingListRow struct {
	Id        string    `json:"id"`
	Name      string    `json:"text"`
	IsStriked bool      `json:"isStriked"`
	Updated   time.Time `json:"updated"`
}

func (row *ShoppingListRow) ETag() string {
	hash, _ := hashstructure.Hash(row, hashstructure.FormatV2, nil)
	return fmt.Sprintf("%v", hash)
}

type ShoppingList struct {
	Id      string            `json:"id"`
	Name    string            `json:"name"`
	Updated time.Time         `json:"updated"`
	Rows    []ShoppingListRow `json:"rows"`
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
	caser := cases.Title(language.Swedish)
	for i, list := range lists {
		for j, row := range list.Rows {
			list.Rows[j].Name = caser.String(row.Name)
		}
		lists[i] = list
	}
	return lists, nil
}

type Suggestion struct {
	Name string `json:"name"`
	Id   int    `json:"id"`
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

type ItemToAdd struct {
	Name    string      `json:"text"`
	Article *Suggestion `json:"article"`
}

func (ica *ICA) AddItem(list ShoppingList, item ItemToAdd) (*ShoppingListRow, error) {
	path := fmt.Sprintf("shopping-list/v1/api/list/%v/row", list.Id)
	data, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	data, err = ica.post(path, data)
	if err != nil {
		return nil, err
	}
	var row ShoppingListRow
	err = json.Unmarshal(data, &row)
	return &row, err
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
