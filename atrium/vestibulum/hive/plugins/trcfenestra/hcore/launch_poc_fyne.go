//go:build fyneboot

package hcore

import (
	"errors"

	"github.com/trimble-oss/tierceron/atrium/speculatio/fenestra/fenestrabase"
)

func LaunchPOCWindow() error {
	launchMu.Lock()
	if launchActive {
		launchMu.Unlock()
		return errors.New("trcfenestra window is already open")
	}
	launchActive = true
	launchMu.Unlock()

	go func() {
		defer func() {
			launchMu.Lock()
			launchActive = false
			launchMu.Unlock()
		}()

		callerCreds := ""
		insecure := true
		headless := true
		serverheadless := false
		env := "QA"

		fenestrabase.CommonMain(
			[]byte{},
			[]byte{},
			[]byte{},
			&callerCreds,
			&insecure,
			&headless,
			&serverheadless,
			&env,
		)
	}()

	return nil
}
