package framework

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// getWebAPIBaseURL returns the web API base url for the given IP.
func getWebAPIBaseURL(hostname string) string {
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
		line := scanner.Bytes()

		// in case of an error there is no Docker prefix
		var bytes []byte
		if len(line) < dockerLogsPrefixLen {
			bytes = append(line, '\n')
		} else {
			bytes = append(line[dockerLogsPrefixLen:], '\n')
		}

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
