package validator_test

import (
	"context"
	"slices"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestWithdrawalDeleteValidator_Validate は、今のパスワードが一致し、退会を元に戻せないことを確認したときだけ受け付け、
// それ以外をそれぞれの欄のエラーにすることを検証する。
func TestWithdrawalDeleteValidator_Validate(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewWithdrawalDeleteValidator(repository.NewUserPasswordRepository(db), repository.NewTradeRepository(db)).WithTx(tx)

	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).Build()
	withoutPasswordID := testutil.NewUserBuilder(t, tx).Build()

	const incorrect = "パスワードが正しくありません"
	const unconfirmed = "退会を元に戻せないことを確認して、チェックを入れてください"

	tests := []struct {
		name          string
		userID        model.UserID
		password      string
		confirmed     bool
		wantPassword  []string
		wantConfirmed []string
	}{
		{name: "パスワードが一致しチェックがある", userID: userID, password: testutil.DefaultBuilderPassword, confirmed: true},
		{name: "パスワードが未入力", userID: userID, password: "", confirmed: true, wantPassword: []string{"入力してください"}},
		{name: "パスワードが違う", userID: userID, password: "wrong-password", confirmed: true, wantPassword: []string{incorrect}},
		{name: "パスワードを持たないユーザー", userID: withoutPasswordID, password: testutil.DefaultBuilderPassword, confirmed: true, wantPassword: []string{incorrect}},
		{name: "チェックが無い", userID: userID, password: testutil.DefaultBuilderPassword, wantConfirmed: []string{unconfirmed}},
		{name: "どちらも誤り", userID: userID, password: "", wantPassword: []string{"入力してください"}, wantConfirmed: []string{unconfirmed}},
	}
	for _, tt := range tests {
		err := v.Validate(ctx, validator.WithdrawalDeleteValidatorInput{UserID: tt.userID, CurrentPassword: tt.password, Confirmed: tt.confirmed})
		if tt.wantPassword == nil && tt.wantConfirmed == nil {
			if err != nil {
				t.Errorf("%s: Validate()のエラー = %v、nilを期待", tt.name, err)
			}
			continue
		}

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Errorf("%s: Validate()のエラー = %v、ValidationErrorを期待", tt.name, err)
			continue
		}
		if got := ve.GetFieldErrors("current_password"); !slices.Equal(got, tt.wantPassword) {
			t.Errorf("%s: current_passwordのエラー = %v、期待値 = %v", tt.name, got, tt.wantPassword)
		}
		if got := ve.GetFieldErrors("confirmed"); !slices.Equal(got, tt.wantConfirmed) {
			t.Errorf("%s: confirmedのエラー = %v、期待値 = %v", tt.name, got, tt.wantConfirmed)
		}
	}
}

// TestWithdrawalDeleteValidator_Validate_TradeInProgress は、返事待ちかマッチ成立の交換があるときは、
// パスワードとチェックがそろっていてもフォーム全体のエラーにし、終わった交換だけなら受け付けることを検証する。
func TestWithdrawalDeleteValidator_Validate_TradeInProgress(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	v := validator.NewWithdrawalDeleteValidator(repository.NewUserPasswordRepository(db), repository.NewTradeRepository(db)).WithTx(tx)

	for _, tt := range []struct {
		status model.TradeStatus
		want   []string
	}{
		{status: model.TradeStatusPending, want: []string{"進行中の交換があるため、退会できません。交換がすべて終わってから、もう一度お試しください"}},
		{status: model.TradeStatusMatched, want: []string{"進行中の交換があるため、退会できません。交換がすべて終わってから、もう一度お試しください"}},
		{status: model.TradeStatusCompleted},
	} {
		userID := testutil.NewUserBuilder(t, tx).Build()
		testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).Build()
		testutil.NewTradeBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), userID).WithStatus(tt.status).Build()

		err := v.Validate(ctx, validator.WithdrawalDeleteValidatorInput{UserID: userID, CurrentPassword: testutil.DefaultBuilderPassword, Confirmed: true})
		if tt.want == nil {
			if err != nil {
				t.Errorf("%s: Validate()のエラー = %v、nilを期待", tt.status, err)
			}
			continue
		}
		if ve := model.AsValidationError(err); ve == nil || !slices.Equal(ve.Global, tt.want) {
			t.Errorf("%s: Validate()のエラー = %v、%v を期待", tt.status, err, tt.want)
		}
	}
}
