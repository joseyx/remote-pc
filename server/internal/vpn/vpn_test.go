package vpn

import (
	"testing"
)

func TestVPNManager(t *testing.T) {
	mgr := NewManager()
	st := mgr.Status()
	t.Logf("Status: %+v", st)
	if !st.Installed {
		t.Skip("Windscribe CLI not installed in current environment")
	}
	t.Logf("Windscribe detected! State=%s, Location=%s, IP=%s, Protocol=%s", st.State, st.Location, st.IP, st.Protocol)
}
