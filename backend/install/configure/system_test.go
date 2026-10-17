package configure

import (
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// TestUncommentMirrors covers the mirrorlist rewrite: the installer ships the
// Arch mirrorlist with every "Server" line commented out and must activate them
// before writing /etc/pacman.d/mirrorlist.
func TestUncommentMirrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "uncomments server lines",
			in:   "#Server = https://a/$repo/os/$arch\n",
			want: "Server = https://a/$repo/os/$arch\n",
		},
		{
			name: "leaves active and unrelated lines alone",
			in:   "# a comment\n#Server = x\nServer = y\n",
			want: "# a comment\nServer = x\nServer = y\n",
		},
		{
			name: "requires the exact commented prefix",
			in:   "#Server=no-space\n#Server = yes\n",
			want: "#Server=no-space\nServer = yes\n",
		},
		{
			name: "empty stays empty",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uncommentMirrors(tt.in); got != tt.want {
				t.Errorf("uncommentMirrors(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestUncommentMirrors_RealMirrorList runs the rewrite over a real embedded
// mirrorlist and asserts every server line ends up active.
func TestUncommentMirrors_RealMirrorList(t *testing.T) {
	content, err := shared.GetEmbeddedMirrorList(shared.DefaultLanguageCode)
	if err != nil {
		t.Fatalf("GetEmbeddedMirrorList: %v", err)
	}
	out := uncommentMirrors(string(content))
	if strings.Contains(out, shared.MirrorListCommentedServerPrefix) {
		t.Error("uncommentMirrors left commented server lines behind")
	}
	if !strings.Contains(out, shared.MirrorListServerPrefix) {
		t.Error("uncommentMirrors produced no active server lines")
	}
}

func TestServicesToEnable(t *testing.T) {
	want := map[string]bool{"NetworkManager": false, "greetd": false, "bluetooth": false}
	seen := map[string]bool{}
	for _, svc := range servicesToEnable {
		if seen[svc] {
			t.Errorf("duplicate service %q", svc)
		}
		seen[svc] = true
		if _, ok := want[svc]; ok {
			want[svc] = true
		}
	}
	for svc, found := range want {
		if !found {
			t.Errorf("expected servicesToEnable to include %q", svc)
		}
	}
}
