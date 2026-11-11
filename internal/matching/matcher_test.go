package matching

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"  Câmera   Canon EOS R6 ": "camera canon eos r6",
		`Monitor LG UltraWide 34"`: "monitor lg ultrawide 34",
		"Tablet iPad Pro 12.9''":   "tablet ipad pro 12 9",
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}
