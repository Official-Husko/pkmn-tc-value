package repository

import "testing"

func TestLookupCardNumberVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "numeric_with_leading_zeros",
			in:   "001",
			want: []string{"001", "1"},
		},
		{
			name: "numeric_without_leading_zeros",
			in:   "1",
			want: []string{"1"},
		},
		{
			name: "alpha_numeric_suffix",
			in:   "SVP 001",
			want: []string{"SVP001", "SVP1"},
		},
		{
			name: "already_trimmed_suffix",
			in:   "GG35",
			want: []string{"GG35"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := lookupCardNumberVariants(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len(lookupCardNumberVariants(%q)) = %d, want %d (%v)", tc.in, len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("lookupCardNumberVariants(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestCardNumberMatchesLookup(t *testing.T) {
	t.Parallel()

	lookup := map[string]struct{}{
		"1": {},
	}
	if !cardNumberMatchesLookup("001", lookup) {
		t.Fatalf("expected 001 to match lookup key 1")
	}

	lookup = map[string]struct{}{
		"SVP1": {},
	}
	if !cardNumberMatchesLookup("SVP 001", lookup) {
		t.Fatalf("expected SVP 001 to match lookup key SVP1")
	}
}
