package session

import (
	"strings"
	"testing"
	"time"
)

const testContinuationKey = "test-continuation-token-key-0123456789"

// TestContinuationManager_verify は、署名した本人の・同じ用途の・期限内のトークンだけを受け付けることを検証する。
func TestContinuationManager_verify(t *testing.T) {
	t.Parallel()

	m := NewContinuationManager(testContinuationKey)
	now := time.Now()
	valid := m.sign(invitationPurpose, "value", now.Add(time.Minute))

	replaceField := func(token string, index int, value string) string {
		parts := strings.Split(token, ".")
		parts[index] = value
		return strings.Join(parts, ".")
	}

	tests := []struct {
		name   string
		m      *ContinuationManager
		token  string
		now    time.Time
		wantOK bool
	}{
		{name: "期限内のトークンを受け付ける", m: m, token: valid, now: now, wantOK: true},
		{name: "期限を過ぎたトークンを拒む", m: m, token: valid, now: now.Add(time.Minute), wantOK: false},
		{name: "値を書き換えたトークンを拒む", m: m, token: replaceField(valid, 2, "other"), now: now, wantOK: false},
		{name: "期限を延ばしたトークンを拒む", m: m, token: replaceField(valid, 3, "9999999999"), now: now, wantOK: false},
		{name: "別の用途で署名したトークンを拒む", m: m, token: m.sign("other_purpose", "value", now.Add(time.Minute)), now: now, wantOK: false},
		{name: "別の鍵で署名したトークンを拒む", m: NewContinuationManager(testContinuationKey + "x"), token: valid, now: now, wantOK: false},
		{name: "区切りの数が違うトークンを拒む", m: m, token: valid + ".extra", now: now, wantOK: false},
		{name: "空のトークンを拒む", m: m, token: "", now: now, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value, ok := tt.m.verify(invitationPurpose, tt.token, tt.now)
			if ok != tt.wantOK {
				t.Fatalf("verify()の結果 = %t、期待値 = %t", ok, tt.wantOK)
			}
			if ok && value != "value" {
				t.Errorf("verify()の値 = %q、期待値 = %q", value, "value")
			}
		})
	}
}
