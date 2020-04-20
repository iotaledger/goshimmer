package framework

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// getWebApiBaseUrl returns the web API base url for the given IP.
func getWebApiBaseUrl(hostname string) string {
	return fmt.Sprintf("http://%s:%s", hostname, apiPort)
}

func createLogFile(name string, logs io.ReadCloser) {
	defer logs.Close()

	f, err := os.Create(fmt.Sprintf("%s%s.log", logsDir, name))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	io.Copy(f, bufio.NewReader(logs))
	f.Sync()
}
