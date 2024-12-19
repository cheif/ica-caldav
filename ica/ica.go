package ica

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mitchellh/hashstructure/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type ICA struct {
	sessionId string
}

func New(sessionId string) ICA {
	return ICA{sessionId}
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

func StartAuth() error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Jar: jar,
	}

	// First we fetch this, to get correct cookies
//	preflightUrl := "https://ims.icagruppen.se/authn/authenticate?serviceProviderId=oauth&forceAuthN=true&resumePath=%2Foauth%2Fv2%2Fauthorize&client_id=ica.se"
    preflightUrl := "https://ims.icagruppen.se/oauth/v2/authorize?client_id=ica.se&response_type=code&scope=openid+ica-se-scope+ica-se-scope-hard&prompt=login&redirect_uri=https://www.ica.se/logga-in/sso/callback"
	resp, err := client.Get(preflightUrl)
	if err != nil {
		return err
	}
	log.Println("resp", resp)

	// Now we want to start BankID
	bankIDStartUrl := "https://ims.icagruppen.se/authn/authenticate/icase-bankid-qr"
	resp, err = client.Get(bankIDStartUrl)
	if err != nil {
		return err
	}

	// Start polling
	err = pollBankID(client)
	if err != nil {
		return err
	}
	log.Println("cookies", jar.Cookies(resp.Request.URL))

	form := url.Values{}
	form.Set("_pollingDone", "true")
	payload := bytes.NewBufferString(form.Encode())
	resp, err = client.Post("https://ims.icagruppen.se/authn/authenticate/icase-bankid-qr/launch", "application/x-www-form-urlencoded", payload)
	if err != nil {
		return err
	}
	log.Println("status", resp.StatusCode)
	log.Println("cookies", jar.Cookies(resp.Request.URL))

	defer resp.Body.Close()
    /*
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
    print("pollingDoneResponse", string(data))
    */
    redirectRequest, err := parseRedirectRequest(resp)
    if err != nil {
        return err
    }
    log.Println("Performing", redirectRequest)
	resp, err = client.Do(redirectRequest)
	if err != nil {
		return err
	}
	log.Println("status", resp.StatusCode)
	log.Println("cookies", jar.Cookies(resp.Request.URL))

	defer resp.Body.Close()
    data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
    log.Println("data", string(data))

	req, err := http.NewRequest("GET", "https://www.ica.se/api/user/information", nil)
	if err != nil {
		return err
	}
	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Println("data", string(data))
	return nil
}

func pollBankID(client *http.Client) error {
	for _ = range 60 {
		bankIDPollUrl := "https://ims.icagruppen.se/authn/authenticate/icase-bankid-qr/wait"
		req, err := http.NewRequest("POST", bankIDPollUrl, nil)
		if err != nil {
		}
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var response bankIDPollResponse
		err = json.Unmarshal(data, &response)
		if err != nil {
			return err
		}
		// {"stopPolling":true,"finishOffUrl":"https://ims.icagruppen.se/authn/authenticate/icase-bankid-qr/wait"}
		log.Println("code", resp.StatusCode)
		log.Println("data", string(data))

		if response.StopPolling {
			req, err := http.NewRequest("POST", response.FinishOffUrl, nil)
			if err != nil {
			}
			req.Header.Set("Accept", "application/json")
			resp, err = client.Do(req)
			if err != nil {
				return err
			}
			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			log.Println("Finished")
			log.Println("data", string(data))
			return nil
		}

		if len(response.Message.QRCode) == 0 {
			// Something went wrong?
			return fmt.Errorf("No QR Code")
		}
		log.Println("QR Code")
		log.Println(response.Message.QRCode)
		time.Sleep(time.Second * 2)
	}
	return nil
}

var formActionRegex regexp.Regexp = *regexp.MustCompile("id=\"form1\" action=\"(.*?)\"")
var tokenRegex regexp.Regexp = *regexp.MustCompile("name=\"token\" value=\"(.*?)\"")
var stateRegex regexp.Regexp = *regexp.MustCompile("name=\"state\" value=\"(.*?)\"")
func parseRedirectRequest(resp *http.Response) (*http.Request, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

    formActionMatches := formActionRegex.FindSubmatch(data)
    if len(formActionMatches) < 2 {
		return nil, fmt.Errorf("Form action not found")
    }
	tokenMatches := tokenRegex.FindSubmatch(data)
	if len(tokenMatches) < 2 {
		return nil, fmt.Errorf("Token not found")
	}

	stateMatches := stateRegex.FindSubmatch(data)
	if len(stateMatches) < 2 {
		return nil, fmt.Errorf("Token not found")
	}
    redirectUrl := resp.Request.URL
    redirectUrl.Path = string(string(formActionMatches[1]))

	form := url.Values{}
	form.Set("token", string(tokenMatches[1]))
	form.Set("state", string(stateMatches[1]))
	payload := bytes.NewBufferString(form.Encode())

    urlString := redirectUrl.String()
    // ICA doesn't support url-encoded `?`, although the spec seems to say it's OK. We just de-encode it.
    urlString = strings.ReplaceAll(urlString, "%3F", "?")

    req, err := http.NewRequest("POST", urlString, payload)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    return req, nil
}

type bankIDPollResponse struct {
	StopPolling  bool                      `json:"stopPolling"`
	FinishOffUrl string                    `json:"finishOffUrl"`
	Message      bankIDPollResponseMessage `json:"message"`
}

type bankIDPollResponseMessage struct {
	QRCode string `json:"qrCode"`
}
