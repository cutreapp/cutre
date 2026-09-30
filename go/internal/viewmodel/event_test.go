package viewmodel_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestEventPeriod は、開催期間を今年の日付でも年を付けて示し、終わりの決まっていないイベントは開始日だけを示すことを検証する。
func TestEventPeriod(t *testing.T) {
	t.Parallel()

	endsOn := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	closed := &model.Event{StartsOn: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), EndsOn: &endsOn}
	openEnded := &model.Event{StartsOn: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}

	tests := []struct {
		name  string
		lang  string
		event *model.Event
		want  string
	}{
		{name: "日本語・終了日あり", lang: i18n.LangJa, event: closed, want: "2026年10月1日〜2026年10月31日"},
		{name: "日本語・終了日なし", lang: i18n.LangJa, event: openEnded, want: "2026年10月1日〜"},
		{name: "英語・終了日あり", lang: i18n.LangEn, event: closed, want: "Oct 1, 2026 – Oct 31, 2026"},
		{name: "英語・終了日なし", lang: i18n.LangEn, event: openEnded, want: "From Oct 1, 2026"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.lang)
			if got := viewmodel.EventPeriod(ctx, tt.event); got != tt.want {
				t.Errorf("EventPeriod() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestNewAdminEvents は、イベントを一覧の行にし、アーカイブしたものに印を付けることを検証する。
func TestNewAdminEvents(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	id := model.EventID(uuid.MustParse("0192f0c6-1234-7abc-8def-0123456789ab"))
	rows := viewmodel.NewAdminEvents(ctx, []*model.Event{
		{ID: id, Name: "秋のくじ", StartsOn: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Status: model.MasterStatusArchived},
	})

	want := viewmodel.AdminEvent{ID: id.String(), Name: "秋のくじ", Period: "2026年10月1日〜", Archived: true}
	if len(rows) != 1 || rows[0] != want {
		t.Errorf("行 = %+v、期待値 = [%+v]", rows, want)
	}
}

// TestNewEventForm は、保存済みのイベントを日付の入力欄の値の形にし、終了日が無ければ空にすることを検証する。
func TestNewEventForm(t *testing.T) {
	t.Parallel()

	endsOn := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	event := &model.Event{Name: "秋のくじ", StartsOn: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), EndsOn: &endsOn, LockVersion: 3}

	if got, want := viewmodel.NewEventForm(event), (viewmodel.EventForm{Name: "秋のくじ", StartsOn: "2026-10-01", EndsOn: "2026-10-31", LockVersion: 3}); got != want {
		t.Errorf("NewEventForm() = %+v、期待値 = %+v", got, want)
	}

	event.EndsOn = nil
	if got := viewmodel.NewEventForm(event); got.EndsOn != "" {
		t.Errorf("終了日が無いときの EndsOn = %q、空を期待", got.EndsOn)
	}
}
