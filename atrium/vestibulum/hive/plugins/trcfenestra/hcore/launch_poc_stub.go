//go:build !fyneboot

package hcore

import "errors"

func LaunchPOCWindow() error {
	return errors.New("trcfenestra is unavailable in this build: missing fyneboot tag")
}
