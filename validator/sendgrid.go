package validator

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/sendgrid/rest"
	sendgrid "github.com/sendgrid/sendgrid-go"
)

// ValidateSendGrid validates the sendgrid API key.
func ValidateSendGrid(apiKey string) (bool, error) {

	host := "https://api.sendgrid.com"
	request := sendgrid.GetRequest(apiKey, "/v3/scopes", host)
	request.Method = "GET"

	sendgrid.DefaultClient = &rest.Client{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   4 * time.Second, // Dial timeout
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DisableKeepAlives:     false,
				MaxConnsPerHost:       10,
			},
		},
	}
	response, err := sendgrid.MakeRequest(request)

	if err != nil {
		return false, err
	}
	if response.StatusCode >= 300 {
		return false, errors.New("error: response code " + strconv.Itoa(response.StatusCode))
	}
	return true, nil
}
