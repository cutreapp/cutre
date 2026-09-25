package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
)

// FlashCookieName はリダイレクトをまたいでフラッシュメッセージを運ぶCookieの名前。
// セッションCookieと同じく __Host- 接頭辞を付け、サブドメインから書き込まれる経路を閉じる。
const FlashCookieName = "__Host-cutre_flash"

// flashCookieMaxAge はフラッシュCookieの有効期間 (秒)。
//
// フラッシュは次のリクエストで読まれて消えるため、本来は寿命を持たなくてよい。
// それでも短い期限を付けるのは、リダイレクト先を開かないままブラウザを閉じた利用者に、
// 後日の再訪で文脈を失ったメッセージが出ることを防ぐため。
const flashCookieMaxAge = 5 * 60

// FlashType はフラッシュメッセージの種類。表示の見た目をこれで切り替える。
type FlashType string

const (
	// FlashSuccess は操作が完了したことを伝えるメッセージ。
	FlashSuccess FlashType = "success"
	// FlashError は操作が失敗したことを伝えるメッセージ。
	FlashError FlashType = "error"
	// FlashWarning は注意を促すメッセージ。
	FlashWarning FlashType = "warning"
	// FlashInfo はお知らせのメッセージ。
	FlashInfo FlashType = "info"
)

// isKnown は表示できる種類かどうかを返す。
// Cookieの値は利用者が書き換えられるため、読み取った種類をここで確かめる。
func (t FlashType) isKnown() bool {
	switch t {
	case FlashSuccess, FlashError, FlashWarning, FlashInfo:
		return true
	default:
		return false
	}
}

// FlashMessage はCookieに載せる1件のフラッシュメッセージ。
type FlashMessage struct {
	Type    FlashType `json:"type"`
	Message string    `json:"message"`
}

// flashContextKey はフラッシュメッセージをリクエストcontextに載せるときのキー。
type flashContextKey struct{}

// FlashManager は短命なCookieでフラッシュメッセージを受け渡す。
//
// フォームの送信結果は303でリダイレクトしてから伝えるため、メッセージはリダイレクトをまたいで
// 運ぶ必要がある。サーバー側に置き場を持たせず、Cookieに載せて次のリクエストで消す。
type FlashManager struct{}

// NewFlashManager は FlashManager を生成する。
func NewFlashManager() *FlashManager {
	return &FlashManager{}
}

// SetSuccess は操作の完了を伝えるフラッシュメッセージを設定する。
func (f *FlashManager) SetSuccess(w http.ResponseWriter, message string) {
	f.set(w, FlashSuccess, message)
}

// SetError は操作の失敗を伝えるフラッシュメッセージを設定する。
func (f *FlashManager) SetError(w http.ResponseWriter, message string) {
	f.set(w, FlashError, message)
}

// SetWarning は注意を促すフラッシュメッセージを設定する。
func (f *FlashManager) SetWarning(w http.ResponseWriter, message string) {
	f.set(w, FlashWarning, message)
}

// SetInfo はお知らせのフラッシュメッセージを設定する。
func (f *FlashManager) SetInfo(w http.ResponseWriter, message string) {
	f.set(w, FlashInfo, message)
}

// Middleware はリクエストのフラッシュメッセージをcontextに載せ、Cookieを消す。
// 一度だけ表示させるため、読み取りと消去を同じ場所で行う。
//
// 消去のSet-Cookieが応答に載るため、利用者に依存しない公開キャッシュ対象の応答
// (静的アセットなど) には掛けない。
func (f *FlashManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flash := f.take(w, r)
		if flash == nil {
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), flashContextKey{}, flash)))
	})
}

// FlashFromContext はctxに載っているフラッシュメッセージを返す。無いときはnilを返す。
func FlashFromContext(ctx context.Context) *FlashMessage {
	flash, _ := ctx.Value(flashContextKey{}).(*FlashMessage)

	return flash
}

// set はフラッシュメッセージをCookieに書き込む。
//
// JSONをbase64urlにするのは、生のJSONがCookieの値に置けない文字 (ダブルクォートなど) を含むため。
// HttpOnlyを付けるのは、メッセージを描画するのがサーバー側のテンプレートであり、
// JavaScriptがCookieを読む必要がないため。
func (f *FlashManager) set(w http.ResponseWriter, flashType FlashType, message string) {
	data, err := json.Marshal(FlashMessage{Type: flashType, Message: message})
	if err != nil {
		slog.Warn("フラッシュメッセージの組み立てに失敗しました", "error", err)
		return
	}

	http.SetCookie(w, flashCookie(base64.RawURLEncoding.EncodeToString(data), flashCookieMaxAge))
}

// take はフラッシュメッセージを読み取り、Cookieを消して返す。
// フラッシュが無いときと、値が壊れていたときはnilを返す (壊れた値はその場で消す)。
func (f *FlashManager) take(w http.ResponseWriter, r *http.Request) *FlashMessage {
	cookie, err := r.Cookie(FlashCookieName)
	if err != nil {
		return nil
	}

	f.clear(w)

	data, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}

	var flash FlashMessage
	if err := json.Unmarshal(data, &flash); err != nil {
		return nil
	}

	// 種類は表示の見た目を決めるため、知らない値をそのまま通さない。
	if !flash.Type.isKnown() || flash.Message == "" {
		return nil
	}

	return &flash
}

// clear はMaxAgeを負にした同名のCookieを送り、ブラウザにフラッシュCookieの削除を指示する。
func (f *FlashManager) clear(w http.ResponseWriter) {
	http.SetCookie(w, flashCookie("", -1))
}

// flashCookie はフラッシュCookieを組み立てる。属性はセッションCookieの方針に揃える。
func flashCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     FlashCookieName,
		Value:    value,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}
