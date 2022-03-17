package validator

import (
	"errors"
	"strconv"

	sendgrid "github.com/sendgrid/sendgrid-go"
)

//ValidateSendGrid validates the sendgrid API key.
func ValidateSendGrid(apiKey string) (bool, error) {

	host := "https://api.sendgrid.com"
	request := sendgrid.GetRequest(apiKey, "/v3/scopes", host)
	request.Method = "GET"
	response, err := sendgrid.API(request)

	if err != nil {
		return false, err
	}
	if response.StatusCode >= 300 {
		return false, errors.New("error: response code " + strconv.Itoa(response.StatusCode))
	}
	return true, nil
}
