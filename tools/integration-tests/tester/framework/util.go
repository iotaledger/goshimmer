package framework

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func getJson(client *http.Client, url string, responseBody interface{}) error {
	r, err := client.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(responseBody)
}

func postJson(client *http.Client, url string, requestBody interface{}, responseBody interface{}) error {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(requestBody)
	if err != nil {
		return err
	}

	response, err := client.Post(url, "application/json", buf)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return json.NewDecoder(response.Body).Decode(responseBody)
}
