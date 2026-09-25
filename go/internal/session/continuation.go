package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/model"
)

// InvitationCookieName は、受け取った招待を登録の手順の間運ぶCookieの名前。
// __Host- 接頭辞を付ける理由はセッションCookie (CookieName) と同じ。
const InvitationCookieName = "__Host-cutre_invitation"

// invitationCookieLifetime は受け取った招待を運ぶCookieの有効期間。
// 登録の手順を1つ進めるたびに発行し直し、各手順を終えるには足りる長さに留める。
// 招待自体の期限と使用状態は、各手順で改めてデータベースに問い合わせて確かめる。
const invitationCookieLifetime = time.Hour

// EmailConfirmationCookieName は、確認コードを送ったメールアドレスの確認を登録の手順の間運ぶCookieの名前。
const EmailConfirmationCookieName = "__Host-cutre_email_confirmation"

// emailConfirmationCookieLifetime は、期限切れのコードからも再送できるよう、招待のCookieと同じ長さにする。
// コード自体の有効期間は email_confirmations.expires_at で確かめる。
const emailConfirmationCookieLifetime = invitationCookieLifetime

// ConfirmedEmailCookieName は、確認を済ませたメールアドレスの確認をアカウントの作成まで運ぶCookieの名前。
// 確認コードを入力する前のCookie (EmailConfirmationCookieName) とは用途を分け、未確認のまま次の手順へ進ませない。
const ConfirmedEmailCookieName = "__Host-cutre_confirmed_email"

// confirmedEmailCookieLifetime は確認を済ませたメールアドレスを運ぶCookieの有効期間。
// 確認した時点から数え直し、アットネームとパスワードを決める時間を残す。
const confirmedEmailCookieLifetime = time.Hour

// PasswordResetCookieName は、パスワードリセットのリンクのトークンを、新しいパスワードの設定まで運ぶCookieの名前。
// リンクのURLからトークンを外すために使い、リンクのトークンそのものではなくその行のIDを入れる。
const PasswordResetCookieName = "__Host-cutre_password_reset"

// passwordResetCookieLifetime はトークンの有効期間に揃える。
// Cookieを発行するのはリンクを開いた時点のため、トークン自体の期限は password_reset_tokens.expires_at で確かめる。
const passwordResetCookieLifetime = model.PasswordResetTokenLifetime

// TwoFactorPendingCookieName は、パスワードを確かめた二要素認証のユーザーを、認証アプリのコードの入力まで運ぶCookieの名前。
// この時点ではセッションを発行せず、コードが合ったときにセッションと引き換える。
const TwoFactorPendingCookieName = "__Host-cutre_two_factor_pending"

// twoFactorPendingCookieLifetime はパスワードを確かめてからコードを入力し終えるまでの時間。
// 認証アプリを開いてコードを写すには足りる長さに留め、盗まれたCookieでコードを試せる時間を短くする。
const twoFactorPendingCookieLifetime = 10 * time.Minute

// continuationTokenVersion は継続トークンの形式の版。形式を変えるときに古いトークンを確実に拒むため値に含める。
const continuationTokenVersion = "v1"

// continuationPurpose は継続トークンをどの手順のために発行したか。
// 署名に含め、ある手順のために発行したトークンを別の手順で使い回されないようにする。
type continuationPurpose string

const (
	invitationPurpose        continuationPurpose = "invitation"
	emailConfirmationPurpose continuationPurpose = "email_confirmation"
	confirmedEmailPurpose    continuationPurpose = "confirmed_email"
	passwordResetPurpose     continuationPurpose = "password_reset"
	twoFactorPendingPurpose  continuationPurpose = "two_factor_pending"
)

// ContinuationManager は、登録やパスワードリセットの途中の状態 (受け取った招待など) を署名付きのCookieで運ぶ。
//
// Cookieに入れるのはデータベースの行のIDと用途と有効期限で、HMAC-SHA-256で署名する。
// IDは読めるが、鍵が無ければ書き換えも新たな発行もできない。
type ContinuationManager struct {
	key []byte
}

// NewContinuationManager は ContinuationManager を生成する。
//
// 鍵が短いときは起動時の設定ミスとしてpanicする。
// config.Load を通らずに組み立てた設定でも、誰でも署名を作れる弱い鍵で動かさないため。
func NewContinuationManager(key string) *ContinuationManager {
	if len(key) < config.ContinuationTokenMinimumKeyLength {
		panic(fmt.Sprintf("継続トークンの鍵には%dバイト以上が必要です", config.ContinuationTokenMinimumKeyLength))
	}

	return &ContinuationManager{key: []byte(key)}
}

// SetInvitationID は受け取った招待のIDをCookieに書き込む。
// 登録の手順を進めたときにも呼び、招待を受け取った時刻によらず次の手順の時間を残す。
func (m *ContinuationManager) SetInvitationID(w http.ResponseWriter, id model.InvitationID) {
	m.setID(w, InvitationCookieName, invitationPurpose, uuid.UUID(id), invitationCookieLifetime)
}

// InvitationID はCookieが運ぶ招待のIDを返す。
// Cookieが無い・形式が違う・署名が合わない・別の用途・期限切れのときは第2の戻り値がfalseになる。
func (m *ContinuationManager) InvitationID(r *http.Request) (model.InvitationID, bool) {
	id, ok := m.id(r, InvitationCookieName, invitationPurpose)
	return model.InvitationID(id), ok
}

// DeleteInvitationID は受け取った招待のCookieを削除する。
func (m *ContinuationManager) DeleteInvitationID(w http.ResponseWriter) {
	http.SetCookie(w, continuationCookie(InvitationCookieName, "", -1))
}

// SetEmailConfirmationID は確認コードを送ったメールアドレスの確認のIDをCookieに書き込む。
// Cookieはコードの期限切れ後も、再送に使える間は残す。
func (m *ContinuationManager) SetEmailConfirmationID(w http.ResponseWriter, id model.EmailConfirmationID) {
	m.setID(w, EmailConfirmationCookieName, emailConfirmationPurpose, uuid.UUID(id), emailConfirmationCookieLifetime)
}

// EmailConfirmationID はCookieが運ぶメールアドレスの確認のIDを返す。
// 読み取れないときの扱いは InvitationID と同じ。
func (m *ContinuationManager) EmailConfirmationID(r *http.Request) (model.EmailConfirmationID, bool) {
	id, ok := m.id(r, EmailConfirmationCookieName, emailConfirmationPurpose)
	return model.EmailConfirmationID(id), ok
}

// DeleteEmailConfirmationID はメールアドレスの確認のCookieを削除する。
func (m *ContinuationManager) DeleteEmailConfirmationID(w http.ResponseWriter) {
	http.SetCookie(w, continuationCookie(EmailConfirmationCookieName, "", -1))
}

// SetConfirmedEmailConfirmationID は確認を済ませたメールアドレスの確認のIDをCookieに書き込む。
func (m *ContinuationManager) SetConfirmedEmailConfirmationID(w http.ResponseWriter, id model.EmailConfirmationID) {
	m.setID(w, ConfirmedEmailCookieName, confirmedEmailPurpose, uuid.UUID(id), confirmedEmailCookieLifetime)
}

// ConfirmedEmailConfirmationID はCookieが運ぶ、確認を済ませたメールアドレスの確認のIDを返す。
// 読み取れないときの扱いは InvitationID と同じ。
func (m *ContinuationManager) ConfirmedEmailConfirmationID(r *http.Request) (model.EmailConfirmationID, bool) {
	id, ok := m.id(r, ConfirmedEmailCookieName, confirmedEmailPurpose)
	return model.EmailConfirmationID(id), ok
}

// DeleteConfirmedEmailConfirmationID は確認を済ませたメールアドレスの確認のCookieを削除する。
func (m *ContinuationManager) DeleteConfirmedEmailConfirmationID(w http.ResponseWriter) {
	http.SetCookie(w, continuationCookie(ConfirmedEmailCookieName, "", -1))
}

// SetPasswordResetTokenID はリンクから受け取ったパスワードリセットのトークンのIDをCookieに書き込む。
func (m *ContinuationManager) SetPasswordResetTokenID(w http.ResponseWriter, id model.PasswordResetTokenID) {
	m.setID(w, PasswordResetCookieName, passwordResetPurpose, uuid.UUID(id), passwordResetCookieLifetime)
}

// PasswordResetTokenID はCookieが運ぶパスワードリセットのトークンのIDを返す。
// 読み取れないときの扱いは InvitationID と同じ。
func (m *ContinuationManager) PasswordResetTokenID(r *http.Request) (model.PasswordResetTokenID, bool) {
	id, ok := m.id(r, PasswordResetCookieName, passwordResetPurpose)
	return model.PasswordResetTokenID(id), ok
}

// DeletePasswordResetTokenID はパスワードリセットのトークンのCookieを削除する。
func (m *ContinuationManager) DeletePasswordResetTokenID(w http.ResponseWriter) {
	http.SetCookie(w, continuationCookie(PasswordResetCookieName, "", -1))
}

// SetTwoFactorPendingUserID は、パスワードを確かめた二要素認証のユーザーのIDをCookieに書き込む。
func (m *ContinuationManager) SetTwoFactorPendingUserID(w http.ResponseWriter, id model.UserID) {
	m.setID(w, TwoFactorPendingCookieName, twoFactorPendingPurpose, uuid.UUID(id), twoFactorPendingCookieLifetime)
}

// TwoFactorPendingUserID はCookieが運ぶ、コードの入力を待っているユーザーのIDを返す。
// 読み取れないときの扱いは InvitationID と同じ。
func (m *ContinuationManager) TwoFactorPendingUserID(r *http.Request) (model.UserID, bool) {
	id, ok := m.id(r, TwoFactorPendingCookieName, twoFactorPendingPurpose)
	return model.UserID(id), ok
}

// DeleteTwoFactorPendingUserID はコードの入力を待っているユーザーのCookieを削除する。
func (m *ContinuationManager) DeleteTwoFactorPendingUserID(w http.ResponseWriter) {
	http.SetCookie(w, continuationCookie(TwoFactorPendingCookieName, "", -1))
}

// setID はIDを用途と期限とともに署名し、Cookieに書き込む。
func (m *ContinuationManager) setID(w http.ResponseWriter, name string, purpose continuationPurpose, id uuid.UUID, lifetime time.Duration) {
	token := m.sign(purpose, id.String(), time.Now().Add(lifetime))

	http.SetCookie(w, continuationCookie(name, token, int(lifetime.Seconds())))
}

// id はCookieの署名・用途・期限を確かめてから、運んでいるIDを返す。
func (m *ContinuationManager) id(r *http.Request, name string, purpose continuationPurpose) (uuid.UUID, bool) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return uuid.UUID{}, false
	}

	value, ok := m.verify(purpose, cookie.Value, time.Now())
	if !ok {
		return uuid.UUID{}, false
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, false
	}

	return id, true
}

// sign は値・用途・有効期限を署名したトークンを返す。
// 区切りに "." を使うため、値に "." を含むものは渡さない (UUIDの文字列表記は含まない)。
func (m *ContinuationManager) sign(purpose continuationPurpose, value string, expiresAt time.Time) string {
	payload := strings.Join([]string{
		continuationTokenVersion,
		string(purpose),
		value,
		strconv.FormatInt(expiresAt.Unix(), 10),
	}, ".")

	return payload + "." + m.signature(payload)
}

// verify はトークンの署名・用途・有効期限を確かめてから値を返す。
// 署名を先に確かめ、改ざんされたトークンの中身を読まないようにする。
func (m *ContinuationManager) verify(purpose continuationPurpose, token string, now time.Time) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 5 {
		return "", false
	}

	payload := strings.Join(parts[:4], ".")
	// hmac.Equal は長さの違いも含めて一定時間で比べ、署名を1バイトずつ当てる攻撃の手がかりを与えない。
	if !hmac.Equal([]byte(parts[4]), []byte(m.signature(payload))) {
		return "", false
	}

	if parts[0] != continuationTokenVersion || parts[1] != string(purpose) {
		return "", false
	}

	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || now.Unix() >= expiresAt {
		return "", false
	}

	return parts[2], true
}

// signature はpayloadのHMAC-SHA-256をbase64urlで返す。
func (m *ContinuationManager) signature(payload string) string {
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write([]byte(payload))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// continuationCookie は継続トークンを運ぶCookieを返す。属性はセッションCookieと揃える。
// サーバー側の状態を運ぶだけで、スクリプトから読む必要が無いためHttpOnlyにする。
func continuationCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}
