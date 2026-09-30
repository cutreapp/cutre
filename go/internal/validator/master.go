package validator

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// masterNameMaxLength はカテゴリー・グッズ・駅の名前の最大の文字数。イベントの名前と同じ長さにする。
const masterNameMaxLength = eventNameMaxLength

// masterPositionMax はカテゴリー・グッズ・駅の並び順に入れられる最大の値。
// 並べ替えは数値の入力で行うため、間を空けて振り直せる余裕を持たせる。
const masterPositionMax = 9999

// archiveMessageMaxLength はマスタをアーカイブする理由の最大の文字数。
const archiveMessageMaxLength = 500

// validateMasterNameAndPosition はカテゴリー・グッズ・駅のフォームに共通する名前と並び順を検証し、保存する値に変換する。
//
// 名前は前後の空白を除いて保存する。並び順は0から masterPositionMax までの整数に限る。
// 誤りは ve に足し、そのときの戻り値は使わない。
func validateMasterNameAndPosition(ctx context.Context, ve *model.ValidationError, name, position string) (string, int32) {
	trimmedName := strings.TrimSpace(name)
	switch {
	case trimmedName == "":
		ve.AddField("name", i18n.T(ctx, "validation_required"))
	case utf8.RuneCountInString(trimmedName) > masterNameMaxLength:
		ve.AddField("name", i18n.T(ctx, "validation_too_long", map[string]any{"Max": masterNameMaxLength}))
	}

	trimmedPosition := strings.TrimSpace(position)
	if trimmedPosition == "" {
		ve.AddField("position", i18n.T(ctx, "validation_required"))
		return trimmedName, 0
	}
	parsed, err := strconv.ParseInt(trimmedPosition, 10, 32)
	if err != nil || parsed < 0 || parsed > masterPositionMax {
		ve.AddField("position", i18n.T(ctx, "validation_position_out_of_range", map[string]any{"Max": masterPositionMax}))
		return trimmedName, 0
	}

	return trimmedName, int32(parsed)
}

// validateArchiveMessage はマスタをアーカイブする理由を検証し、前後の空白を除いた理由を返す。
// あとから見た運営が経緯を追えるよう、理由を必須にする。入力の誤りは *model.ValidationError で返す。
func validateArchiveMessage(ctx context.Context, archiveMessage string) (string, error) {
	ve := model.NewValidationError()
	message := strings.TrimSpace(archiveMessage)

	switch {
	case message == "":
		ve.AddField("archive_message", i18n.T(ctx, "validation_required"))
	case utf8.RuneCountInString(message) > archiveMessageMaxLength:
		ve.AddField("archive_message", i18n.T(ctx, "validation_too_long", map[string]any{"Max": archiveMessageMaxLength}))
	}

	if ve.HasErrors() {
		return "", ve
	}

	return message, nil
}
