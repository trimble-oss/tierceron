package common

import "testing"

func TestResolveTrcshTalkModePrefersTrcshTalkMode(t *testing.T) {
	config := map[string]interface{}{
		CfgTrcshTalkMode: ModeHub,
		CfgMode:          ModeTalkback,
	}

	if got := resolveTrcshTalkMode(&config); got != ModeHub {
		t.Fatalf("resolveTrcshTalkMode() = %q, want %q", got, ModeHub)
	}
}

func TestResolveTrcshTalkModeFallsBackToMode(t *testing.T) {
	config := map[string]interface{}{
		CfgMode: ModeTalkback,
	}

	if got := resolveTrcshTalkMode(&config); got != ModeTalkback {
		t.Fatalf("resolveTrcshTalkMode() = %q, want %q", got, ModeTalkback)
	}
}

func TestResolveTrcshTalkModeUsesHubClient(t *testing.T) {
	config := map[string]interface{}{
		CfgTrcshTalkMode: ModeHubClient,
	}

	if got := resolveTrcshTalkMode(&config); got != ModeHubClient {
		t.Fatalf("resolveTrcshTalkMode() = %q, want %q", got, ModeHubClient)
	}
}

func TestResolveTrcshTalkModeUsesHub(t *testing.T) {
	config := map[string]interface{}{
		CfgTrcshTalkMode: ModeHub,
	}

	if got := resolveTrcshTalkMode(&config); got != ModeHub {
		t.Fatalf("resolveTrcshTalkMode() = %q, want %q", got, ModeHub)
	}
}

func TestResolveTrcshTalkModeDefaultsToTalkbackForUnsupportedMode(t *testing.T) {
	config := map[string]interface{}{
		CfgTrcshTalkMode: "trcshtalk",
	}

	if got := resolveTrcshTalkMode(&config); got != ModeTalkback {
		t.Fatalf("resolveTrcshTalkMode() = %q, want %q", got, ModeTalkback)
	}
}
