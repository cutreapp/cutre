package testutil

import "context"

// FakeTurnstileVerifier は turnstile.Verifier のテストダブル。
// Cloudflareのsiteverifyを呼ばずに決めた結果を返し、ハンドラーのテストで
// 通過・非通過・検証エラーの各経路を実際の通信なしに通せるようにする。
//
// Verifyのシグネチャを合わせて構造的に turnstile.Verifier を満たし、
// testutilから turnstile パッケージを参照せずに済ませる。
type FakeTurnstileVerifier struct {
	// Passed はVerifyが返す通過の結果。
	Passed bool

	// Err は非nilのときVerifyが返すerror。siteverifyの拒否やシステム障害の経路を検証するのに使う。
	Err error

	// Token は最後にVerifyへ渡されたトークン。
	// ハンドラーが送信された cf-turnstile-response を渡したことをテストで確かめるのに使う。
	Token string
}

// Verify はトークンを記録し、決めておいた Passed と Err を返す。
func (f *FakeTurnstileVerifier) Verify(_ context.Context, token string) (bool, error) {
	f.Token = token
	return f.Passed, f.Err
}
