package model_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TestAnonymizedEmailAndAtname は、退会したユーザーの匿名の値がユーザーごとに異なり、
// 登録で使える値と重ならない形 (届かないドメイン・アットネームに使えない文字) であることを検証する。
func TestAnonymizedEmailAndAtname(t *testing.T) {
	t.Parallel()

	id := model.UserID(uuid.MustParse("0192f0c6-1234-7abc-8def-0123456789ab"))
	other := model.UserID(uuid.MustParse("0192f0c6-1234-7abc-8def-0123456789ac"))

	if got, want := model.AnonymizedEmail(id), "deleted-0192f0c6-1234-7abc-8def-0123456789ab@deleted.invalid"; got != want {
		t.Errorf("AnonymizedEmail = %q、期待値 = %q", got, want)
	}
	if got, want := model.AnonymizedAtname(id), "deleted-0192f0c6-1234-7abc-8def-0123456789ab"; got != want {
		t.Errorf("AnonymizedAtname = %q、期待値 = %q", got, want)
	}
	if model.AnonymizedEmail(id) == model.AnonymizedEmail(other) || model.AnonymizedAtname(id) == model.AnonymizedAtname(other) {
		t.Error("別のユーザーの匿名の値が重なった")
	}

	// 登録のアットネームの規則 (validator の atnameRegex と同じ) に合わないこと。
	if regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(model.AnonymizedAtname(id)) {
		t.Error("AnonymizedAtname が登録で使えるアットネームの形になっている")
	}
	if !strings.HasSuffix(model.AnonymizedEmail(id), ".invalid") {
		t.Error("AnonymizedEmail のドメインが予約されたTLDの .invalid でない")
	}
}
