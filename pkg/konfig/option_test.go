package konfig

import (
	"os"
	"testing"
)

func TestNewBootstrapOptionFromEnvFlag(t *testing.T) {
	_ = os.Setenv("CONF_IP", "localhost")
	_ = os.Setenv("CONF_PORT", "28500")
	opt := NewBootstrapOptionFromEnvFlag()
	if len(opt.Addrs) != 1 && opt.Addrs[0] != "localhost:28500" {
		t.Fatal("unexpected address option")
	}
}
