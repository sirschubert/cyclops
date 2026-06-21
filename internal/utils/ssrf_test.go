package utils

import "testing"

func TestIsMetadataHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"169.254.169.254", true},
		{"169.254.169.254:80", true},
		{"169.254.0.1", true},
		{"metadata.google.internal", true},
		{"METADATA.GOOGLE.INTERNAL", true},
		{"fe80::1", true},
		{"93.184.216.34", false},
		{"example.com", false},
		{"8.8.8.8:53", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsMetadataHost(c.host); got != c.want {
			t.Errorf("IsMetadataHost(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestMetadataDialControl(t *testing.T) {
	if err := MetadataDialControl("tcp", "169.254.169.254:80", nil); err == nil {
		t.Error("expected metadata address to be blocked at dial")
	}
	if err := MetadataDialControl("tcp", "93.184.216.34:443", nil); err != nil {
		t.Errorf("expected public address to be allowed, got %v", err)
	}
}
