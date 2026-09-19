//go:build !darwin

package gbh

import (
	"errors"

	"github.com/sqweek/dialog"
)

func chooseROMFile() (string, error) {
	filename, err := dialog.File().
		Title("Open ROM").
		Filter("Game Boy ROM", "gb", "gbc").
		Filter("All files", "*").
		Load()
	if errors.Is(err, dialog.ErrCancelled) {
		return "", errFileDialogCancelled
	}
	return filename, err
}
