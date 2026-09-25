// Package turnstile はCloudflare TurnstileによるBot対策の、サーバー側の検証を提供する。
package turnstile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// siteverifyURL はCloudflare Turnstileのサーバー側検証エンドポイント。
	siteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	// requestTimeout はsiteverifyへの呼び出しの上限。
	// 応答が返らないときに、検証を待つハンドラーが止まり続けないようにする。
	requestTimeout = 10 * time.Second

	// maxResponseBytes はsiteverifyの応答として読み込む上限。
	// 応答は数百バイトのJSONのため、想定外に大きな応答をメモリへ読み込まないよう抑える。
	maxResponseBytes = 64 << 10
)

// Verifier はTurnstileのレスポンストークンを検証する。
//
// ハンドラーは実装ではなくこのインターフェースを受け取り、テストでは
// testutil.FakeTurnstileVerifier に差し替えてsiteverifyを呼ばずに各経路を検証する。
type Verifier interface {
	// Verify はtokenがTurnstileのチャレンジを通過したかを返す。
	//
	// siteverifyに問い合わせるまでもない非通過 (空のトークン) では (false, nil) を返す。
	// 検証の拒否 (siteverifyの success:false) とシステム障害 (通信・デコード・200以外) では
	// 非nilのerrorを返す。呼び出し側は false とerrorのどちらも非通過として扱う。
	Verify(ctx context.Context, token string) (bool, error)
}

// Client はCloudflareのsiteverify APIでTurnstileのトークンを検証する。Verifierを実装する。
type Client struct {
	secretKey  string
	httpClient *http.Client
}

var _ Verifier = (*Client)(nil)

// verifyRequest はsiteverify APIへ送るリクエストボディ。
type verifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
}

// verifyResponse はsiteverify APIのレスポンスのうち、検証に使う部分。
type verifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// NewClient はsecretKeyでsiteverifyに認証するClientを作る。
//
// 持つのはシークレットキーだけにする。サイトキーは検証に使わず、設定からテンプレートへ直接渡す。
// secretKeyが空のClientはすべてのトークンを通す (Turnstileを無効にした開発・テスト環境向け)。
func NewClient(secretKey string) *Client {
	return &Client{
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// Verify はtokenをsiteverify APIに照合し、通過したかを返す。
func (c *Client) Verify(ctx context.Context, token string) (bool, error) {
	// シークレットキーが空なのはTurnstileを無効にした状態。
	// 設定はサイトキーとそろって空であることを保証しており、ウィジェットも描画されないため、
	// トークンを求めずにすべてのリクエストを通す。
	if c.secretKey == "" {
		return true, nil
	}

	// トークンが空なのは、ウィジェットを解かずにフォームが送信された場合
	// (JavaScriptのブロックやBotによる直接のPOST)。システムの異常ではない想定内の非通過のため、
	// errorを返さず、ログの重さの判断をハンドラーに委ねる。
	if token == "" {
		return false, nil
	}

	// siteverify APIは仕様上リクエストボディにシークレットを含めるため、
	// gosecのG117 (シークレットらしいフィールドのシリアライズ) はここでは誤検知。
	//nolint:gosec // G117
	body, err := json.Marshal(verifyRequest{
		Secret:   c.secretKey,
		Response: token,
	})
	if err != nil {
		return false, fmt.Errorf("siteverifyへのリクエストボディのエンコードに失敗: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, siteverifyURL, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("siteverifyへのリクエストの作成に失敗: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("siteverifyへのリクエストの送信に失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return false, fmt.Errorf("siteverifyのレスポンスの読み込みに失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("siteverifyがエラーを返しました (ステータスコード: %d): %s", resp.StatusCode, respBody)
	}

	var result verifyResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return false, fmt.Errorf("siteverifyのレスポンスのデコードに失敗: %w", err)
	}

	if !result.Success {
		// 拒否の理由をハンドラーがログに残せるよう、エラーコードをerrorに含める。
		return false, fmt.Errorf("turnstileの検証で拒否されました (エラーコード: %v)", result.ErrorCodes)
	}

	return true, nil
}
