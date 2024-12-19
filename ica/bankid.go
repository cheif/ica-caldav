package ica

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

type BankIDAuthenticator struct {
	client *http.Client
	ica    *ICA
}

func NewBankIDAuthentication(ica *ICA) BankIDAuthenticator {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	client := &http.Client{
		Jar: jar,
	}
	return BankIDAuthenticator{client, ica}
}

func (a *BankIDAuthenticator) Start() error {
	// First we do a preflight, to set some cookies and get redirected properly
	preflightUrl := "https://ims.icagruppen.se/oauth/v2/authorize?client_id=ica.se&response_type=code&scope=openid+ica-se-scope+ica-se-scope-hard&prompt=login&redirect_uri=https://www.ica.se/logga-in/sso/callback"

	// FIXME: Probably check status-codes as well?
	_, err := a.client.Get(preflightUrl)
	if err != nil {
		return err
	}

	// Now we want to start BankID
	bankIDStartUrl := "https://ims.icagruppen.se/authn/authenticate/icase-bankid-qr"
	_, err = a.client.Get(bankIDStartUrl)
	return err
}

func (a *BankIDAuthenticator) State() (isFinished bool, qrCode string, err error) {
	bankIDPollUrl := "https://ims.icagruppen.se/authn/authenticate/icase-bankid-qr/wait"
	req, err := http.NewRequest("POST", bankIDPollUrl, nil)
	if err != nil {
	}
	req.Header.Set("Accept", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return false, "", err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}
	var response bankIDPollResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return false, "", err
	}

	if !response.StopPolling {
		if len(response.Message.QRCode) == 0 {
			// Something went wrong?
			return false, "", fmt.Errorf("No QR Code")
		}
		return false, response.Message.QRCode, nil
	}

	// We're finished, fetch the real token
	session, err := a.finish()
	if err != nil {
		return false, "", err
	}

	// Set the session on the actual ica-session
	a.ica.setSessionId(session)
	return true, "", nil
}

func (a *BankIDAuthenticator) finish() (string, error) {
	// Post that we're done, this will return us a html-form with some important values
	form := url.Values{}
	form.Set("_pollingDone", "true")
	payload := bytes.NewBufferString(form.Encode())
	resp, err := a.client.Post("https://ims.icagruppen.se/authn/authenticate/icase-bankid-qr/launch", "application/x-www-form-urlencoded", payload)
	if err != nil {
		return "", err
	}

	// Parse out URL/form data from the above form, so that we can POST it
	redirectRequest, err := parseRedirectRequest(resp)
	if err != nil {
		return "", err
	}

	resp, err = a.client.Do(redirectRequest)
	if err != nil {
		return "", err
	}

	// Now we should be done, and have a valid `thSession` cookie
	for _, cookie := range a.client.Jar.Cookies(resp.Request.URL) {
		if cookie.Name == "thSessionId" {
			return cookie.Value, nil
		}
	}
	// No valid cookie, error out
	return "", fmt.Errorf("No valid cookie returned")
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
	StopPolling bool                      `json:"stopPolling"`
	Message     bankIDPollResponseMessage `json:"message"`
}

type bankIDPollResponseMessage struct {
	QRCode string `json:"qrCode"`
}
