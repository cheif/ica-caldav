package ica

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
)

func TestParseResponseCode(t *testing.T) {
    data, err := os.ReadFile("testdata/redirect.html")
    if err != nil {
        t.Error(err)
    }
    originalReq, _ := http.NewRequest("POST", "https://ims.icagruppen.se", nil)
    resp := http.Response {
        Request: originalReq,
        Body: io.NopCloser(bytes.NewBuffer(data)),
    }
    req, err := parseRedirectRequest(&resp)
    if err != nil {
        t.Error(err)
    }

    if req.URL.String() != "https://ims.icagruppen.se/oauth/v2/authorize?forceAuthN=true&client_id=ica.se" {
        t.Errorf("Incorrect redirect URL: %v", req.URL)
    }


    data, _ = io.ReadAll(req.Body)

    formParams, _ := url.ParseQuery(string(data))
    if formParams.Get("token") != "8hSasBGitfMmDqc9QNJOMnk0ELQ3FObp" {
        t.Errorf("Incorrect token in body: %v", string(data))
    }
    if formParams.Get("state") != "R_SUA0ql5ThIxVohZ7y7UHYC1F3LM6qpTZ" {
        t.Errorf("Incorrect state in body: %v", string(data))
    }

    if req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
        t.Errorf("Incorrect Content-Type: %v", req.Header)
    }
}
