package middleware

import "testing"

func TestSanitizeRequestID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ab12cd34", "ab12cd34"},   // 正常 id 原样
		{"  ab12  ", "ab12"},       // 空格剥除
		{"a b\tc\nd", "abcd"},      // 空格/制表/换行(防注入)剥除
		{"héllo", "hllo"},          // 非 ASCII 剥除
		{"", ""},                    // 空 → 空
	}
	for _, c := range cases {
		if got := sanitizeRequestID(c.in); got != c.want {
			t.Errorf("sanitizeRequestID(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// 超长截断到 64。
	long := make([]byte, 100)
	for i := range long {
		long[i] = 'x'
	}
	if got := sanitizeRequestID(string(long)); len(got) != 64 {
		t.Errorf("length cap: got len %d, want 64", len(got))
	}
}
