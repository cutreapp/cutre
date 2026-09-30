package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestUnarchiveEventUsecase_Execute は、編集者がアーカイブしたイベントを公開に戻して理由を空にでき、
// 版が違うときと公開中のイベントには AppErrCodeConflict を、一般のユーザーには AppErrCodeForbidden を返すことを検証する。
func TestUnarchiveEventUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEventRepository(db).WithTx(tx)
	uc := usecase.NewUnarchiveEventUsecase(repo)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	eventID := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()

	assertAppErrorCode(t, uc.Execute(t.Context(), usecase.UnarchiveEventInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventID: eventID}), model.AppErrCodeForbidden)

	assertAppErrorCode(t, uc.Execute(t.Context(), usecase.UnarchiveEventInput{User: editor, EventID: eventID, LockVersion: 1}), model.AppErrCodeConflict)

	if err := uc.Execute(t.Context(), usecase.UnarchiveEventInput{User: editor, EventID: eventID}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	found, _ := repo.FindByID(t.Context(), eventID)
	if found.Status != model.MasterStatusPublished || found.ArchiveMessage != nil {
		t.Errorf("元に戻したイベント = %+v、公開中・理由なしを期待", found)
	}

	// 状態の条件だけで拒むことを確かめるため、今の版を送る。
	assertAppErrorCode(t, uc.Execute(t.Context(), usecase.UnarchiveEventInput{User: editor, EventID: eventID, LockVersion: 1}), model.AppErrCodeConflict)
	if found, _ := repo.FindByID(t.Context(), eventID); found.LockVersion != 1 {
		t.Errorf("版 = %d、公開中のイベントを戻そうとしても版1のままを期待", found.LockVersion)
	}
}
