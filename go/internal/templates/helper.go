// Package templates はtemplテンプレートから呼び出すヘルパーを提供する。
package templates

import (
	"context"
	"net/url"

	"github.com/cutreapp/cutre/go/internal/i18n"
)

// T はctxのロケールでmessageIDを翻訳する。
// テンプレートがi18nではなくtemplatesパッケージに依存するようにするためのi18n.Tの薄いラッパー。
func T(ctx context.Context, messageID string, templateData ...map[string]any) string {
	return i18n.T(ctx, messageID, templateData...)
}

// Locale はctxのロケールを返す。html要素のlang属性に使う。
// lang属性が取るのはBCP 47の言語タグで、ロケールはその綴りをそのまま持つため値は加工せずマークアップへ入る。
func Locale(ctx context.Context) string {
	return i18n.GetLocale(ctx)
}

// RootPath はctxのロケールの言語版のトップページのパスを返す。
// 言語版ごとにトップページのアドレスが違うため、行き先をテンプレートに直書きすると
// 英語版から日本語版のトップページへ送ることになる。
func RootPath(ctx context.Context) string {
	return i18n.LocalePath(i18n.GetLocale(ctx), "/")
}

// ReturnToParam はログイン後の戻り先を運ぶクエリパラメータとフォームの値の名前。
// ログインを求めるミドルウェアが付け、ログイン画面のフォームが運び、ログインの処理が読むため、ここに1つだけ置く。
const ReturnToParam = "return_to"

// WithReturnTo はパスにログイン後の戻り先をクエリで付けて返す。戻り先が空ならパスをそのまま返す。
// ログインの手順の画面の間で戻り先を引き継ぐのに使う。returnTo には検証済みの値だけを渡す。
func WithReturnTo(path, returnTo string) string {
	if returnTo == "" {
		return path
	}

	return path + "?" + url.Values{ReturnToParam: {returnTo}}.Encode()
}

// SignInPath はログイン画面のパス (言語コードを含まない形)。
const SignInPath = "/sign_in"

// SignInTwoFactorPath は、ログインで認証アプリのコードを入力する画面のパス (言語コードを含まない形)。
// パスワードを確かめた二要素認証のユーザーを、ログイン画面の言語版のまま送る。
const SignInTwoFactorPath = "/sign_in/two_factor"

// SignInTwoFactorRecoveryPath は、ログインでリカバリーコードを入力する画面のパス (言語コードを含まない形)。
const SignInTwoFactorRecoveryPath = "/sign_in/two_factor/recovery"

// HelpContactURL は、WikinoにあるCutreのヘルプのお問い合わせのページのURL。
// 認証アプリもリカバリーコードも無くしてログインできない人を案内する。ページを用意するまでの仮の値。
const HelpContactURL = "https://wikino.app/s/cutre"

// HomePath はログイン後のホームのパス。
// ログイン後のページは表示言語を users.locale で決めるため、言語版を持たない。
const HomePath = "/home"

// UserSessionPath はログイン中のセッションのパス。DELETEでログアウトする。
const UserSessionPath = "/user_session"

// SignUpPath は登録 (メールアドレスの入力) の画面のパス (言語コードを含まない形)。
const SignUpPath = "/sign_up"

// EmailConfirmationPath は確認コードの入力画面のパス (言語コードを含まない形)。
const EmailConfirmationPath = "/email_confirmation"

// AccountPath はアカウントの作成 (アットネームとパスワードの設定) の画面のパス (言語コードを含まない形)。
const AccountPath = "/account"

// PasswordResetPath はパスワードリセットの申請の画面のパス (言語コードを含まない形)。
const PasswordResetPath = "/password_reset"

// PasswordResetSentPath はパスワードリセットの申請を受け付けた後の画面のパス (言語コードを含まない形)。
const PasswordResetSentPath = "/password_reset/sent"

// PasswordPath は新しいパスワードを設定する画面のパス (言語コードを含まない形)。
// パスワードリセットのメールのリンクが、トークンをクエリに付けてここを指す。
const PasswordPath = "/password"

// InvitationPath は招待リンクのパスを返す。
// QRコードを粗く保てるよう、パスを短くしている。
func InvitationPath(token string) string {
	return "/i/" + token
}

// ProfilePath はアットネームのユーザーのプロフィールのパスを返す。
// 先頭の @ で他のパスと区別するため、アットネームにルーティングと衝突する予約語を設けずに済む。
func ProfilePath(atname string) string {
	return "/@" + atname
}

// SettingsInvitationPath は招待の画面のパス。ログイン後のページのため言語版を持たない。
const SettingsInvitationPath = "/settings/invitation"

// SettingsTwoFactorAuthPath は二要素認証の画面のパス。POSTで有効にする。ログイン後のページのため言語版を持たない。
const SettingsTwoFactorAuthPath = "/settings/two_factor_auth"

// NewSettingsTwoFactorAuthPath は二要素認証を有効にする (認証アプリへ登録する) 画面のパス。
const NewSettingsTwoFactorAuthPath = "/settings/two_factor_auth/new"

// SettingsMessageConsentPath はメッセージの利用 (メッセージの取り扱いへの同意) の画面のパス。
// POSTで同意し、DELETEで同意をやめる。ログイン後のページのため言語版を持たない。
const SettingsMessageConsentPath = "/settings/message_consent"

// SettingsPlacesPath は交換場所の画面のパス。PATCHで保存する。ログイン後のページのため言語版を持たない。
const SettingsPlacesPath = "/settings/places"

// SettingsWithdrawalPath は退会の画面のパス。DELETEで退会する。ログイン後のページのため言語版を持たない。
const SettingsWithdrawalPath = "/settings/withdrawal"

// AdminPath は管理画面の入口のパス。編集者と管理者だけが開け、それ以外の人には存在しないページとして扱う。
// ログイン後のページのため言語版を持たない。
const AdminPath = "/admin"

// AdminEventsPath は管理画面のイベントの一覧のパス。POSTでイベントを作成する。
const AdminEventsPath = "/admin/events"

// NewAdminEventPath は管理画面のイベントの作成の画面のパス。
const NewAdminEventPath = "/admin/events/new"

// AdminEventPath は管理画面のイベントのパスを返す。PATCHで更新し、DELETEで削除する。
func AdminEventPath(id string) string {
	return AdminEventsPath + "/" + id
}

// EditAdminEventPath は管理画面のイベントの編集の画面のパスを返す。
func EditAdminEventPath(id string) string {
	return AdminEventPath(id) + "/edit"
}

// AdminEventArchivePath は管理画面のイベントのアーカイブのパスを返す。POSTでアーカイブし、DELETEで元に戻す。
func AdminEventArchivePath(id string) string {
	return AdminEventPath(id) + "/archive"
}

// NewAdminEventArchivePath は管理画面のイベントのアーカイブの画面 (理由の入力) のパスを返す。
func NewAdminEventArchivePath(id string) string {
	return AdminEventArchivePath(id) + "/new"
}

// AdminEventCategoriesPath は管理画面のイベント eventID のカテゴリーのパスを返す。POSTでイベントの配下にカテゴリーを作成する。
// カテゴリーの一覧はイベントの編集の画面に並べるため、このパスはGETを持たない。
func AdminEventCategoriesPath(eventID string) string {
	return AdminEventPath(eventID) + "/categories"
}

// NewAdminEventCategoryPath は管理画面のイベント eventID の配下にカテゴリーを作成する画面のパスを返す。
func NewAdminEventCategoryPath(eventID string) string {
	return AdminEventCategoriesPath(eventID) + "/new"
}

// AdminEventCategoryPath は管理画面のカテゴリーのパスを返す。PATCHで更新し、DELETEで削除する。
// カテゴリーのIDだけでカテゴリーを決められるため、イベントのIDをパスに含めない。
func AdminEventCategoryPath(id string) string {
	return "/admin/categories/" + id
}

// EditAdminEventCategoryPath は管理画面のカテゴリーの編集の画面のパスを返す。
func EditAdminEventCategoryPath(id string) string {
	return AdminEventCategoryPath(id) + "/edit"
}

// AdminEventCategoryArchivePath は管理画面のカテゴリーのアーカイブのパスを返す。POSTでアーカイブし、DELETEで元に戻す。
func AdminEventCategoryArchivePath(id string) string {
	return AdminEventCategoryPath(id) + "/archive"
}

// NewAdminEventCategoryArchivePath は管理画面のカテゴリーのアーカイブの画面 (理由の入力) のパスを返す。
func NewAdminEventCategoryArchivePath(id string) string {
	return AdminEventCategoryArchivePath(id) + "/new"
}

// AdminEventCategoryGoodsPath は管理画面のカテゴリー eventCategoryID のグッズのパスを返す。POSTでカテゴリーの配下にグッズを作成する。
// グッズの一覧はカテゴリーの編集の画面に並べるため、このパスはGETを持たない。
func AdminEventCategoryGoodsPath(eventCategoryID string) string {
	return AdminEventCategoryPath(eventCategoryID) + "/goods"
}

// NewAdminGoodsPath は管理画面のカテゴリー eventCategoryID の配下にグッズを作成する画面のパスを返す。
func NewAdminGoodsPath(eventCategoryID string) string {
	return AdminEventCategoryGoodsPath(eventCategoryID) + "/new"
}

// AdminGoodsPath は管理画面のグッズのパスを返す。PATCHで更新し、DELETEで削除する。
// グッズのIDだけでグッズを決められるため、カテゴリーとイベントのIDをパスに含めない。
func AdminGoodsPath(id string) string {
	return "/admin/goods/" + id
}

// EditAdminGoodsPath は管理画面のグッズの編集の画面のパスを返す。
func EditAdminGoodsPath(id string) string {
	return AdminGoodsPath(id) + "/edit"
}

// AdminGoodsArchivePath は管理画面のグッズのアーカイブのパスを返す。POSTでアーカイブし、DELETEで元に戻す。
func AdminGoodsArchivePath(id string) string {
	return AdminGoodsPath(id) + "/archive"
}

// NewAdminGoodsArchivePath は管理画面のグッズのアーカイブの画面 (理由の入力) のパスを返す。
func NewAdminGoodsArchivePath(id string) string {
	return AdminGoodsArchivePath(id) + "/new"
}

// AdminStationsPath は管理画面の駅の一覧のパス。POSTで駅を作成する。
const AdminStationsPath = "/admin/stations"

// NewAdminStationPath は管理画面の駅の作成の画面のパス。
const NewAdminStationPath = "/admin/stations/new"

// NewAdminStationInPrefecturePath は、都道府県 prefectureCode を選んだ状態の駅の作成の画面のパスを返す。
func NewAdminStationInPrefecturePath(prefectureCode string) string {
	return NewAdminStationPath + "?" + url.Values{"prefecture_code": {prefectureCode}}.Encode()
}

// AdminStationPath は管理画面の駅のパスを返す。PATCHで更新し、DELETEで削除する。
func AdminStationPath(id string) string {
	return AdminStationsPath + "/" + id
}

// EditAdminStationPath は管理画面の駅の編集の画面のパスを返す。
func EditAdminStationPath(id string) string {
	return AdminStationPath(id) + "/edit"
}

// AdminStationArchivePath は管理画面の駅のアーカイブのパスを返す。POSTでアーカイブし、DELETEで元に戻す。
func AdminStationArchivePath(id string) string {
	return AdminStationPath(id) + "/archive"
}

// NewAdminStationArchivePath は管理画面の駅のアーカイブの画面 (理由の入力) のパスを返す。
func NewAdminStationArchivePath(id string) string {
	return AdminStationArchivePath(id) + "/new"
}

// EventsPath はユーザー向けのイベントの一覧のパス。リストに追加するグッズを、イベントからたどる入口。
const EventsPath = "/events"

// EventPath はユーザー向けのイベントのカテゴリーの一覧のパスを返す。
func EventPath(id string) string {
	return EventsPath + "/" + id
}

// EventCategoryPath は、イベント eventID のカテゴリーのグッズの一覧のパスを返す。
func EventCategoryPath(eventID, id string) string {
	return EventPath(eventID) + "/categories/" + id
}

// MatchesPath はマッチ候補の画面のパス。ログイン後のページのため言語版を持たない。
const MatchesPath = "/matches"

// ItemsPath はリストのアイテムのパス。POSTでリストに追加する。
const ItemsPath = "/items"

// NewItemPath はリストに追加する画面のパス。追加するグッズとリストはクエリで選ぶ (NewItemForGoodsPath)。
const NewItemPath = ItemsPath + "/new"

// NewItemForGoodsPath は、グッズ goodsID をリスト kind に追加する画面のパスを返す。
func NewItemForGoodsPath(goodsID, kind string) string {
	return NewItemPath + "?" + url.Values{"goods_id": {goodsID}, "kind": {kind}}.Encode()
}

// ItemPath はリストのアイテムのパスを返す。PATCHで数量とひとことを更新し、DELETEでリストから外す。
func ItemPath(id string) string {
	return ItemsPath + "/" + id
}

// EditItemPath はリストのアイテムの編集の画面のパスを返す。
func EditItemPath(id string) string {
	return ItemPath(id) + "/edit"
}

// ListPath はユーザー向けのリストのパス。メインメニューの行き先で、譲れるリストを出す。
// ほしいリストはクエリで切り替える (ListKindPath)。
const ListPath = "/list"

// ListKindPath はリスト kind を出すリストのパスを返す。
func ListKindPath(kind string) string {
	return ListPath + "?" + url.Values{"kind": {kind}}.Encode()
}

// TradesPath は交換のパス。メインメニューの行き先で、進行中の交換とマッチ候補を出す。POSTで交換を申し込む。
const TradesPath = "/trades"

// TradeHistoryPath はこれまでの交換のパス。終わった交換を出す。
const TradeHistoryPath = TradesPath + "/history"

// TradePath は交換のページのパスを返す。
func TradePath(id string) string {
	return TradesPath + "/" + id
}

// TradeWithdrawalPath は交換の申し込みの取り下げのパスを返す。POSTで取り下げる。
func TradeWithdrawalPath(id string) string {
	return TradePath(id) + "/withdrawal"
}

// TradeApprovalPath は交換の申し込みの承認のパスを返す。POSTで承認する。
func TradeApprovalPath(id string) string {
	return TradePath(id) + "/approval"
}

// TradeDeclinePath は交換の申し込みのお断りのパスを返す。GETで理由とひとことを入れる画面を開き、POSTでお断りする。
func TradeDeclinePath(id string) string {
	return TradePath(id) + "/decline"
}

// TradeCompletionPath は「交換できた」のパスを返す。GETでひとことを入れる画面を開き、POSTで記録する。
func TradeCompletionPath(id string) string {
	return TradePath(id) + "/completion"
}

// TradeFailurePath は「交換できなかった」のパスを返す。GETで理由とひとことを入れる画面を開き、POSTで記録する。
func TradeFailurePath(id string) string {
	return TradePath(id) + "/failure"
}

// TradeCancellationPath は交換をやめるパスを返す。GETで理由とひとことを入れる画面を開き、POSTで交換をやめる。
func TradeCancellationPath(id string) string {
	return TradePath(id) + "/cancellation"
}

// TradeMessagesPath は交換のメッセージのページのパスを返す。POSTでメッセージを送る。
func TradeMessagesPath(id string) string {
	return TradePath(id) + "/messages"
}

// TradeMessageRetractionPath は交換 tradeID のメッセージ messageID の取り消しのパスを返す。POSTで取り消す。
func TradeMessageRetractionPath(tradeID, messageID string) string {
	return TradeMessagesPath(tradeID) + "/" + messageID + "/retraction"
}

// MessagesPath はメッセージの一覧のパス。メインメニューの行き先で、交換ごとのメッセージを出す。
const MessagesPath = "/messages"

// TradeMessagesLatestAnchor は、交換のメッセージのページの流れの末尾に置く要素のid。
// 送ったあとや交換のページから開いたときに、最新のメッセージまで進めた位置で開くためのアンカーにする。
const TradeMessagesLatestAnchor = "trade-messages-latest"

// TradeMessagesLatestPath は、交換のメッセージのページを流れの末尾まで進めた位置で開くパスを返す。
func TradeMessagesLatestPath(id string) string {
	return TradeMessagesPath(id) + "#" + TradeMessagesLatestAnchor
}

// NewTradePath は、アットネーム atname のユーザーに交換を申し込む組み合わせを選ぶ画面のパスを返す。
func NewTradePath(atname string) string {
	return ProfilePath(atname) + "/trades/new"
}

// NewTradeConfirmationPath は、アットネーム atname のユーザーへの申し込み内容を確認する画面のパスを返す。
// 選んだ組み合わせはクエリで受け取る。組み合わせを選ぶ画面のフォームがGETでここへ送る。
func NewTradeConfirmationPath(atname string) string {
	return NewTradePath(atname) + "/confirmation"
}

// NewTradeWithItemsPath は、もらうもの receiveItemIDs と渡すもの giveItemIDs を選んだ状態で、組み合わせを選ぶ画面を開くパスを返す。
// 申し込み内容の確認から、組み合わせを変えに戻るのに使う。
func NewTradeWithItemsPath(atname string, receiveItemIDs, giveItemIDs []string) string {
	return NewTradePath(atname) + "?" + url.Values{"receive_item_ids": receiveItemIDs, "give_item_ids": giveItemIDs}.Encode()
}

// LocalePath は言語コードを含まないパスを、ctxのロケールの言語版のパスへ変換する。
// フォームの送信先のように、表示中の言語版に留まるべき行き先を組み立てるのに使う。
func LocalePath(ctx context.Context, path string) string {
	return i18n.LocalePath(i18n.GetLocale(ctx), path)
}
