package policy_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
)

// TestAdminPolicy は、管理画面を編集者と管理者に、マスタの削除を管理者だけに許すことを検証する。
func TestAdminPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		user             *model.User
		wantUseAdmin     bool
		wantDeleteMaster bool
	}{
		{name: "ログインしていない", user: nil, wantUseAdmin: false, wantDeleteMaster: false},
		{name: "一般", user: &model.User{Role: model.UserRoleUser}, wantUseAdmin: false, wantDeleteMaster: false},
		{name: "編集者", user: &model.User{Role: model.UserRoleEditor}, wantUseAdmin: true, wantDeleteMaster: false},
		{name: "管理者", user: &model.User{Role: model.UserRoleAdmin}, wantUseAdmin: true, wantDeleteMaster: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := policy.NewAdminPolicy(tt.user)
			if got := p.CanUseAdmin(); got != tt.wantUseAdmin {
				t.Errorf("CanUseAdmin() = %v、期待値 = %v", got, tt.wantUseAdmin)
			}
			if got := p.CanDeleteMaster(); got != tt.wantDeleteMaster {
				t.Errorf("CanDeleteMaster() = %v、期待値 = %v", got, tt.wantDeleteMaster)
			}
		})
	}
}
