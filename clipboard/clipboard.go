package clipboard

import (
	"os/exec"
)

func ToClipboard(s string) error {
	copyCmd := exec.Command("pbcopy")
	in, err := copyCmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := copyCmd.Start(); err != nil {
		return err
	}
	if _, err := in.Write([]byte(s)); err != nil {
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}

	return copyCmd.Wait()

}
