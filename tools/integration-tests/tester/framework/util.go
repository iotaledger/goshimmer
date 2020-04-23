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

// createLogFile creates a log file from the given logs ReadCloser.
func createLogFile(name string, logs io.ReadCloser) error {
	defer logs.Close()

	f, err := os.Create(fmt.Sprintf("%s%s.log", logsDir, name))
	if err != nil {
		return err
	}
	defer f.Close()

	// remove non-ascii chars at beginning of line
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		bytes := append(scanner.Bytes()[dockerLogsPrefixLen:], '\n')
		_, err = f.Write(bytes)
		if err != nil {
			return err
		}
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	return nil
}
