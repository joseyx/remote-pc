package vpn

import (
	"testing"
)

func TestVPNManager(t *testing.T) {
	mgr := NewManager()
	st := mgr.Status()
	t.Logf("Status: %+v", st)
	if !st.Installed {
		t.Fatalf("Expected Windscribe CLI to be detected")
	}
	t.Logf("Windscribe detected! State=%s, Location=%s, IP=%s, Protocol=%s", st.State, st.Location, st.IP, st.Protocol)
}
