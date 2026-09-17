package middleware

import (
	"io"
	"net/http"
)

// redirectByName はリダイレクトを発行したソフトウェアとしてCutreを示す名前。
//
// バージョンやルートではなく安定した製品名にするのは、どの層が応答したかを運用者に伝えつつ、
// 変動するデプロイの詳細を公開しないためである。
const redirectByName = "cutre"

// RedirectBy はアプリケーションが書き出すリダイレクトに、その発行元を示す
// Redirect-By ヘッダーを付ける。
//
// 本番のインスタンスはCloudflareとDokku内蔵のプロキシの後ろに置かれ、いずれの層も
// 自身でリダイレクトを発行しうる。3xxだけではどの層が書いたのかが分からないため、
// 意図せず転送されるURLを追うには3つの設定を読むことになる。
// 発行元をレスポンスで示せば、1ホップにつき1つのヘッダーを読むだけで済む。
func RedirectBy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&redirectByWriter{ResponseWriter: w}, r)
	})
}

// redirectByWriter はステータス行がまだ送出されていない間にヘッダーを足す。
// 事後にステータスを読むのではなくWriteHeaderで捕まえるのは、
// ステータスを送出したあとに設定したヘッダーがクライアントに届かないためである。
type redirectByWriter struct {
	http.ResponseWriter
}

// WriteHeader はリダイレクトに、そしてリダイレクトにだけ発行元を示す。
//
// 3xxにLocationを求めるのは、このヘッダーが「このレスポンスはクライアントを別の場所へ送る」と
// 主張するものだからである。304 Not Modifiedはクライアントをどこへも送らない3xxであり、
// これに印を付けると主張が偽になる。
// ステータスを設定せず本文を書くハンドラーは200を送りここを通らないが、それにもヘッダーは要らない。
func (w *redirectByWriter) WriteHeader(status int) {
	if status >= http.StatusMultipleChoices && status < http.StatusBadRequest &&
		w.Header().Get("Location") != "" {
		w.Header().Set("Redirect-By", redirectByName)
	}

	w.ResponseWriter.WriteHeader(status)
}

// Unwrap は下層のResponseWriterを返す。
// http.ResponseController が、サーバー自身のResponseWriterが実装する追加のインターフェース
// (Flusher・Hijacker・デッドラインの設定など) へ到達するための手段である。
// このラッパーは全ルートを覆うため、これが無いとそれらがどこでも利かなくなる。
func (w *redirectByWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// ReadFrom は本文の書き出しを下層のResponseWriterに任せる。
//
// 下層が io.ReaderFrom を実装していても、インターフェースの埋め込みでは昇格せず io.Copy から見えなくなる。
// net/httpのResponseWriterはそこでsendfileを使い、静的アセットの配信がこの判定を通るため、
// 橋渡ししないとファイルの内容をバッファ経由でコピーすることになる。
//
// 本文を書くレスポンスはステータスを持たないか200であり、
// ここがWriteHeaderを経なくても発行元を示す判定は変わらない。
func (w *redirectByWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	return io.Copy(w.ResponseWriter, r)
}
