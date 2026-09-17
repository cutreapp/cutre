package middleware

import (
	"io"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/config"
)

// assetCacheControl は配信に成功した静的アセットに与えるキャッシュ方針。
//
// テンプレートがアセットを参照するURLは ?v= にアセットバージョンを持ち、
// 中身が変わる (= デプロイする) と参照するURLも変わる (config.Config.AssetVersion)。
// 保存されたレスポンスが新しい内容を覆い隠すことがないため、再検証を求めずに長く持たせられる。
const assetCacheControl = "public, max-age=31536000, immutable"

// devAssetCacheControl は開発環境の静的アセットに与えるキャッシュ方針。
//
// 開発環境のアセットバージョンは呼び出しごとに変わるミリ秒のタイムスタンプで (config.Config.AssetVersion)、
// 保存しても同じURLが二度要求されない。再利用されない表現でブラウザのキャッシュを埋めない。
const devAssetCacheControl = "no-store"

// assetErrorCacheControl は配信に失敗した静的アセットに与えるキャッシュ方針。
// 存在しないアセットへの応答を保存させないのは、後からそのアドレスに置いたファイルを覆い隠さないため。
const assetErrorCacheControl = "private, no-store"

// AssetCache は静的アセットの配信にキャッシュ方針を与える。
// /static/* のルートにだけ登録し、アプリケーションが描画するページには関与しない。
func AssetCache(cfg *config.Config) func(http.Handler) http.Handler {
	cacheControl := assetCacheControl
	if cfg.IsDev() {
		cacheControl = devAssetCacheControl
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", cacheControl)

			next.ServeHTTP(&assetCacheWriter{ResponseWriter: w}, r)
		})
	}
}

// assetCacheWriter は配信に失敗したときだけ、先に書き込まれた方針を差し替える。
//
// 成功時の値を先に書いておくのは、http.FileServer がステータスを設定せずに本文を書く経路でも
// 方針が付くようにするためである。
type assetCacheWriter struct {
	http.ResponseWriter
}

// WriteHeader はアセットを配信しなかった応答の方針を、保存させないものへ差し替える。
//
// 206 Partial Content を成功と同じ扱いにするのは、これが200と同じ表現の一部を運ぶ応答だからである。
// http.FileServer はRangeヘッダーを持つリクエストにこれで応じる。
// ここでno-storeを返すと、受け取った断片を保存できず、続きを取得して組み立てることもできなくなる。
//
// 304 Not Modified を成功と同じ扱いにするのは、これが保存済みのアセットをそのまま使ってよいという
// 応答だからである。キャッシュは304のヘッダーで保存済みレスポンスを更新するため、
// ここでno-storeを返すと、有効なはずのアセットを取り下げさせることになる。
func (w *assetCacheWriter) WriteHeader(status int) {
	if status != http.StatusOK && status != http.StatusPartialContent && status != http.StatusNotModified {
		w.Header().Set("Cache-Control", assetErrorCacheControl)
	}

	w.ResponseWriter.WriteHeader(status)
}

// Unwrap は下層のResponseWriterを返す。
// http.ResponseController が、サーバー自身のResponseWriterが実装する追加のインターフェース
// (Flusher・Hijacker・デッドラインの設定など) へ到達するための手段である。
func (w *assetCacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// ReadFrom は本文の書き出しを下層のResponseWriterに任せる。
//
// 下層が io.ReaderFrom を実装していても、インターフェースの埋め込みでは昇格せず io.Copy から見えなくなる。
// net/httpのResponseWriterはそこでsendfileを使い、アセットの配信がこの判定を通るため、
// 橋渡ししないとファイルの内容をバッファ経由でコピーすることになる。
func (w *assetCacheWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	return io.Copy(w.ResponseWriter, r)
}
