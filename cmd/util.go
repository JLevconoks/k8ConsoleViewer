package cmd

import (
	"github.com/pkg/errors"
	"log"
	"os"
	"path/filepath"
)

func getAppDir() (string, error) {
	s, err := os.Executable()
	if err != nil {
		return "", errors.Wrapf(err, "Error in os.Executable() %v", s)
	}
	symlink, err := filepath.EvalSymlinks(s)
	if err != nil {
		return "", errors.Wrapf(err, "Error in filepath.EvalSymlinks() %v", s)
	}

	return filepath.Dir(symlink), nil
}

func logToFile() {
	appDir, err := getAppDir()
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.OpenFile(appDir+"/log.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(file)
}
