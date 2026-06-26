package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePushEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		// Recognized push services (exact host and subdomains).
		{"fcm", "https://fcm.googleapis.com/fcm/send/abc123", false},
		{"legacy gcm", "https://android.googleapis.com/gcm/send/abc", false},
		{"firefox autopush", "https://updates.push.services.mozilla.com/wpush/v2/abc", false},
		{"safari", "https://web.push.apple.com/abc", false},
		{"wns", "https://db5.notify.windows.com/w/?token=abc", false},

		// Rejected.
		{"http scheme", "http://fcm.googleapis.com/fcm/send/abc", true},
		{"loopback ip literal", "https://127.0.0.1/x", true},
		{"metadata ip literal", "https://169.254.169.254/latest/meta-data", true},
		{"private ip literal", "https://10.1.2.3/x", true},
		{"unknown host", "https://evil.example/x", true},
		{"suffix-spoof host", "https://fcm.googleapis.com.evil.example/x", true},
		{"prefix-spoof host", "https://evilfcm.googleapis.com/x", true},
		{"empty host", "https:///path", true},
		{"garbage", "://not a url", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePushEndpoint(tc.endpoint)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
