package validator

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// placeNoteMaxLength は交換場所の「ほかに出られるところ」の最大の文字数。相手が読むひとことのため、アイテムのひとことと揃える。
const placeNoteMaxLength = 200

// PlaceUpdateValidator は交換場所のフォームの形を検証する。
// 選んだ駅が今も選べるかは、駅をロックしたあとに PlaceStationUpdateValidator で確かめる。
type PlaceUpdateValidator struct{}

// NewPlaceUpdateValidator は PlaceUpdateValidator を生成する。
func NewPlaceUpdateValidator() *PlaceUpdateValidator {
	return &PlaceUpdateValidator{}
}

// PlaceUpdateValidatorInput は PlaceUpdateValidator.Validate の入力。フォームの値をそのまま受け取る。
type PlaceUpdateValidatorInput struct {
	// StationIDs は選んだ駅のIDのチェックボックスの値。
	StationIDs []string
	PlaceNote  string
}

// PlaceUpdateValidateOutput は PlaceUpdateValidator.Validate の結果。
type PlaceUpdateValidateOutput struct {
	// StationIDs は選んだ駅のID。同じ駅を2度送られても1つにまとめる。
	StationIDs []model.StationID
	// PlaceNote は前後の空白を除いた「ほかに出られるところ」。
	PlaceNote string
}

// Validate は交換場所のフォームを検証し、保存する駅と「ほかに出られるところ」を返す。
//
// 駅は選択肢から選ぶため、IDとして読めない値は選べない駅と同じエラーにする。
// 駅を1つも選ばないことは許す。交換場所をすべて外せるようにするため。
// 「ほかに出られるところ」は任意で、placeNoteMaxLength 文字までに限る。入力の誤りは *model.ValidationError で返す。
func (v *PlaceUpdateValidator) Validate(ctx context.Context, input PlaceUpdateValidatorInput) (*PlaceUpdateValidateOutput, error) {
	ve := model.NewValidationError()

	stationIDs := make([]model.StationID, 0, len(input.StationIDs))
	seen := make(map[model.StationID]bool, len(input.StationIDs))
	for _, raw := range input.StationIDs {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			ve.AddGlobal(i18n.T(ctx, "settings_place_station_unavailable"))
			break
		}
		id := model.StationID(parsed)
		if !seen[id] {
			seen[id] = true
			stationIDs = append(stationIDs, id)
		}
	}

	placeNote := strings.TrimSpace(input.PlaceNote)
	if utf8.RuneCountInString(placeNote) > placeNoteMaxLength {
		ve.AddField("place_note", i18n.T(ctx, "validation_too_long", map[string]any{"Max": placeNoteMaxLength}))
	}

	if ve.HasErrors() {
		return nil, ve
	}

	return &PlaceUpdateValidateOutput{StationIDs: stationIDs, PlaceNote: placeNote}, nil
}

// PlaceStationUpdateValidator は、交換場所に選んだ駅が今も選べるかを検証する。
type PlaceStationUpdateValidator struct {
	stationRepo *repository.StationRepository
}

// NewPlaceStationUpdateValidator は PlaceStationUpdateValidator を生成する。
func NewPlaceStationUpdateValidator(stationRepo *repository.StationRepository) *PlaceStationUpdateValidator {
	return &PlaceStationUpdateValidator{stationRepo: stationRepo}
}

// WithTx はtx内で駅を読む新しいValidatorを返す。
func (v *PlaceStationUpdateValidator) WithTx(tx *sql.Tx) *PlaceStationUpdateValidator {
	return &PlaceStationUpdateValidator{stationRepo: v.stationRepo.WithTx(tx)}
}

// Validate は、ユーザー userID が交換場所に選んだ駅 stationIDs がどれも選べるかを確かめる。
//
// 選べるのは公開中の駅と、アーカイブしていても既に選んでいる駅。アーカイブしたマスタを、
// すでに使っているところではそのまま使えるようにするため。無い駅と削除した駅は選べない。
// 画面を開いたあとにアーカイブ・削除されたときに起きるため、選び直しを案内する *model.ValidationError を返す。
// 駅の状態が確かめたあとに変わらないよう、呼び出し側が駅をロックしてから呼ぶ。
func (v *PlaceStationUpdateValidator) Validate(ctx context.Context, userID model.UserID, stationIDs []model.StationID) error {
	if len(stationIDs) == 0 {
		return nil
	}

	stations, err := v.stationRepo.ListByIDs(ctx, stationIDs)
	if err != nil {
		return fmt.Errorf("交換場所に選んだ駅の取得に失敗: %w", err)
	}
	current, err := v.stationRepo.ListByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("交換場所の駅の取得に失敗: %w", err)
	}
	selected := make(map[model.StationID]bool, len(current))
	for _, station := range current {
		selected[station.ID] = true
	}

	selectable := 0
	for _, station := range stations {
		if station.Status == model.MasterStatusPublished || (station.IsArchived() && selected[station.ID]) {
			selectable++
		}
	}
	if selectable != len(stationIDs) {
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "settings_place_station_unavailable"))
		return ve
	}

	return nil
}
