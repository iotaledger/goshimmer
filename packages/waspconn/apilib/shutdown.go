package apilib

import (
	"fmt"
	"net/http"
)

func Shutdown(host string) error {
	_, err := http.Get(fmt.Sprintf("http://%s/adm/shutdown", host))
	return err
}
