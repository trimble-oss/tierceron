//go:build !talkbackservicelocal
// +build !talkbackservicelocal

package coreopts

func IsTrcshTalkBackLocal() bool {
	return false
}
