package workflow

import "testing"

// TestResolveDepth locks S4: quality tiers map to bounded depth profiles.
func TestResolveDepth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		quality string
		want    DepthProfile
	}{
		{"balanced", DepthProfile{VerifyVotes: 1, FanOutCap: 5, Synthesis: false}},
		{"ultra", DepthProfile{VerifyVotes: 3, FanOutCap: 5, Synthesis: true}},
		{"unknown", DepthProfile{VerifyVotes: 1, FanOutCap: 5, Synthesis: false}},
		{"", DepthProfile{VerifyVotes: 1, FanOutCap: 5, Synthesis: false}},
	}
	for _, c := range cases {
		if got := ResolveDepth(c.quality); got != c.want {
			t.Errorf("ResolveDepth(%q) = %+v, want %+v", c.quality, got, c.want)
		}
	}
}

// TestValidateDepthCaps locks the hard ceilings: over-cap values are rejected,
// within-cap values pass.
func TestValidateDepthCaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                          string
		verifyVotes, fanOutCap, retry int
		wantErr                       bool
	}{
		{"verify over cap", 4, 0, 0, true},
		{"fan-out over cap", 0, 6, 0, true},
		{"retry over cap", 0, 0, 4, true},
		{"all at cap", MaxVerifyVotes, MaxFanOut, MaxRetry, false},
		{"all zero", 0, 0, 0, false},
	}
	for _, c := range cases {
		err := validateDepthCaps("review", c.verifyVotes, c.fanOutCap, c.retry)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: validateDepthCaps err = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}
