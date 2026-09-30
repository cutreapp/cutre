package model_test

import (
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestMessageConsent_IsValid は、今の版の文面に同意していて、やめていない同意だけを有効とすることを検証する。
func TestMessageConsent_IsValid(t *testing.T) {
	t.Parallel()

	withdrawnAt := time.Now()
	tests := []struct {
		name    string
		consent model.MessageConsent
		want    bool
	}{
		{name: "今の版でやめていない", consent: model.MessageConsent{Version: model.CurrentMessageConsentVersion}, want: true},
		{name: "今の版でやめた", consent: model.MessageConsent{Version: model.CurrentMessageConsentVersion, WithdrawnAt: &withdrawnAt}},
		{name: "古い版", consent: model.MessageConsent{Version: model.CurrentMessageConsentVersion - 1}},
	}
	for _, tt := range tests {
		if got := tt.consent.IsValid(); got != tt.want {
			t.Errorf("%s: IsValid() = %v、期待値 = %v", tt.name, got, tt.want)
		}
	}
}
