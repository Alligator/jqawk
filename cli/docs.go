package cli

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/browser"
)

//go:embed docs/jqawk.1
var docMan []byte

//go:embed docs/jqawk.html
var docHtml []byte

func PrintDocs() error {
	// do we have man?
	_, err := exec.LookPath("man")
	if err == nil {
		f, err := os.CreateTemp("", "jqawk.*.1")
		if err != nil {
			return err
		}

		path := f.Name()
		defer os.Remove(path)

		if _, err := f.Write(docMan); err != nil {
			f.Close()
			return err
		}

		if err := f.Close(); err != nil {
			return err
		}

		cmd := exec.Command("man", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	// no man, try to open a browser
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(cacheDir, "jqawk"), 0o755); err != nil {
		return err
	}

	htmlPath := filepath.Join(cacheDir, "jqawk", "jqawk.html")
	if err := os.WriteFile(htmlPath, docHtml, 0o644); err != nil {
		return err
	}

	if err := browser.OpenFile(htmlPath); err != nil {
		return err
	}

	return nil
}
