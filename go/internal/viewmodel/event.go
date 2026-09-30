package viewmodel

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// dateInputLayout は日付の入力欄 (<input type="date">) の値の形。
const dateInputLayout = "2006-01-02"

// EventPeriod はイベントの開催期間を、表示中のロケールの書式で返す。
//
// 開催期間は暦日のため、タイムゾーンを変換せずにそのまま書式にする。
// イベントは年をまたいで並ぶため、今年の日付でも年を省かない。
func EventPeriod(ctx context.Context, event *model.Event) string {
	layout := i18n.T(ctx, "date_year_month_day_layout")
	startsOn := event.StartsOn.Format(layout)
	if event.EndsOn == nil {
		return i18n.T(ctx, "event_period_open_ended", map[string]any{"StartsOn": startsOn})
	}

	return i18n.T(ctx, "event_period", map[string]any{"StartsOn": startsOn, "EndsOn": event.EndsOn.Format(layout)})
}

// AdminEvent は管理画面のイベントの一覧の1行。
type AdminEvent struct {
	ID       string
	Name     string
	Period   string
	Archived bool
}

// NewAdminEvents はイベントを管理画面の一覧の行にする。
func NewAdminEvents(ctx context.Context, events []*model.Event) []AdminEvent {
	rows := make([]AdminEvent, len(events))
	for i, event := range events {
		rows[i] = AdminEvent{
			ID:       event.ID.String(),
			Name:     event.Name,
			Period:   EventPeriod(ctx, event),
			Archived: event.IsArchived(),
		}
	}

	return rows
}

// EventForm は管理画面のイベントの作成・編集のフォームに入れる値。
// 日付は入力欄の値の形 (2006-01-02) の文字列で持ち、エラーで描き直すときは送られた値をそのまま戻す。
type EventForm struct {
	Name     string
	StartsOn string
	EndsOn   string
	// LockVersion は編集のフォームが持ち回るイベントの版。作成のフォームでは使わない。
	LockVersion int32
}

// NewEventForm は保存済みのイベントを、編集のフォームに入れる値にする。
func NewEventForm(event *model.Event) EventForm {
	form := EventForm{
		Name:        event.Name,
		StartsOn:    event.StartsOn.Format(dateInputLayout),
		LockVersion: event.LockVersion,
	}
	if event.EndsOn != nil {
		form.EndsOn = event.EndsOn.Format(dateInputLayout)
	}

	return form
}

// EventRow はユーザー向けのイベントの一覧の1行。
type EventRow struct {
	ID     string
	Name   string
	Period string
	// GoodsCount はイベントの公開中のグッズの種類の数。
	GoodsCount int64
	// Quantities はイベントの中で、ユーザーがリストに入れたアイテムの数量の合計。
	Quantities model.ItemQuantities
}

// NewEventRows はイベントをユーザー向けの一覧の行にする。
// goodsCounts と quantities に無いイベントは、グッズやアイテムが無いものとして0にする。
func NewEventRows(ctx context.Context, events []*model.Event, goodsCounts map[model.EventID]int64, quantities map[model.EventID]model.ItemQuantities) []EventRow {
	rows := make([]EventRow, len(events))
	for i, event := range events {
		rows[i] = EventRow{
			ID:         event.ID.String(),
			Name:       event.Name,
			Period:     EventPeriod(ctx, event),
			GoodsCount: goodsCounts[event.ID],
			Quantities: quantities[event.ID],
		}
	}

	return rows
}
