package ica

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

type Cache interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte) error
}

type BankIDAuthenticator struct {
	sessionCache Cache
	client       *http.Client
}

func NewBankIDAuthentication(cache Cache) BankIDAuthenticator {
	jar := createCookieJar(cache)
	client := &http.Client{
		Jar: jar,
	}
	return BankIDAuthenticator{cache, client}
}

func createCookieJar(cache Cache) http.CookieJar {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	// Try reading cookies from session file
	slog.Info("Starting read")
	data, err := cache.ReadFile("session.json")
	slog.Info("Read done")
	if err != nil {
		slog.Info("No cached session found", err)
		return jar
	}
	var cookies []*http.Cookie
	err = json.Unmarshal(data, &cookies)
	if err != nil {
		slog.Info("Corrupt cache found", err)
		return jar
	}
	icaURL, err := url.Parse("https://www.ica.se")
	if err != nil {
		return jar
	}
	jar.SetCookies(icaURL, cookies)
	return jar
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

func (a *BankIDAuthenticator) Poll() (isFinished bool, qrCode string, err error) {
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

	// We're done polling, finish up
	err = a.finish()
	if err != nil {
		return false, "", err
	}
	return true, "", nil
}

func (a *BankIDAuthenticator) finish() error {
	// Post that we're done, this will return us a html-form with some important values
	form := url.Values{}
	form.Set("_pollingDone", "true")
	payload := bytes.NewBufferString(form.Encode())
	resp, err := a.client.Post("https://ims.icagruppen.se/authn/authenticate/icase-bankid-qr/launch", "application/x-www-form-urlencoded", payload)
	if err != nil {
		return err
	}

	// Parse out URL/form data from the above form, so that we can POST it
	redirectRequest, err := parseRedirectRequest(resp)
	if err != nil {
		return err
	}

	_, err = a.client.Do(redirectRequest)
	if err != nil {
		return err
	}

	// Verify that we have a valid session
	if !a.HasValidSession() {
		return fmt.Errorf("No valid session")
	}

	icaURL, err := url.Parse("https://www.ica.se")
	if err == nil {
		// Write cookies to session file
		cookies := a.client.Jar.Cookies(icaURL)
		data, err := json.Marshal(cookies)
		if err == nil {
			err = a.sessionCache.WriteFile("session.json", data)
			if err != nil {
				slog.Error("Error writing cache", err)
			}
		}
	}

	return err
}

func (a *BankIDAuthenticator) HasStarted() bool {
    if a.HasValidSession() {
        return true
    } else {
        imsURL, err := url.Parse("https://ims.icagruppen.se")
        if err != nil {
            return false
        }
        cookies := a.client.Jar.Cookies(imsURL)
        return len(cookies) != 0
    }
}

func (a *BankIDAuthenticator) HasValidSession() bool {
	_, err := a.getSessionValue()
	return err == nil
}

func (a *BankIDAuthenticator) GetSession() (*ICA, error) {
	session, err := a.getSessionValue()
	if err != nil {
		return nil, err
	}
	return &ICA{sessionId: session}, nil
}

func (a *BankIDAuthenticator) getSessionValue() (string, error) {
	// Now we should be done, and have a valid `thSession` cookie
	icaURL, err := url.Parse("https://www.ica.se")
	if err != nil {
		return "", err
	}
	for _, cookie := range a.client.Jar.Cookies(icaURL) {
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
