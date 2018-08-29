package util

import "os"

func PathExists(path string) (bool) {
	_, err := os.Stat(path)

	if err != nil {
		return false
	}

	return true

}
