package middleware

import (
	"io"
	"mime"
	"net/http"
	"strings"
)

// htmlCacheControl はキャッシュ方針を自分で決めなかったHTMLレスポンスに与える既定値。
//
// privateは、共有キャッシュが1人分の表現を他の訪問者へ配らないようにする。
// 言語版の案内は選択のCookieでも変わるため、そちらをVaryで列挙する代わりにここで共有を断つ。
// no-cacheは保存を許したうえで再検証を求める意味で、デプロイした内容がすぐ行き渡る。
const htmlCacheControl = "private, no-cache"

// varyAcceptLanguage はHTMLの表現がブラウザの言語設定で変わることを示すVaryのフィールド名。
// 別の言語版があることを知らせる案内を出すかどうかがこのヘッダーで決まる (internal/middleware/i18n.go)。
const varyAcceptLanguage = "Accept-Language"

// HTMLCache はHTMLのレスポンスに、その表現がブラウザの言語設定で変わることと、
// 自分で方針を決めなかった場合のキャッシュ方針を示す。
//
// 対象をルートではなくレスポンスの型で見分けるのは、/static/* にまでVaryが付くと
// アセットの長期キャッシュのキーが訪問者の言語設定ごとに分岐してしまうためである。
// ミドルウェアチェーンでは末尾スラッシュの正規化より外側に置き、そこが出す301のHTMLも覆う。
func HTMLCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&htmlCacheWriter{ResponseWriter: w}, r)
	})
}

// htmlCacheWriter はステータス行が送出される前に、HTMLかどうかを見てヘッダーを足す。
// 応答の型はハンドラーが書き出すまで分からないため、判定を応答の直前まで遅らせる。
type htmlCacheWriter struct {
	http.ResponseWriter

	// decided は応答の型を見分けて判定を終えたかどうか。
	// ステータスと本文のどちらが先に来ても、足すのは1回だけにする。
	decided bool
}

// decide はHTMLのレスポンスにVaryと既定のキャッシュ方針を書き込む。
// bodyはこれから書き出す本文で、ステータス行だけを送るときはnilを渡す。
func (w *htmlCacheWriter) decide(body []byte) {
	if w.decided {
		return
	}
	w.decided = true

	header := w.Header()

	mediaType, _, err := mime.ParseMediaType(responseContentType(header, body))
	if err != nil || mediaType != "text/html" {
		return
	}

	// 既存の値を置き換えずに足す。圧縮のように表現を変える要因が後から入ったとき、
	// そちらの列挙を落とさないため (複数行のVaryは1つのリストとして解釈される)。
	// 既に列挙されていれば足さない。同じ名前を重ねてもキャッシュのキーは変わらず、
	// 列挙を読む側に同じ応答が違う応答として見えるだけになる。
	if !varyHasField(header, varyAcceptLanguage) {
		header.Add("Vary", varyAcceptLanguage)
	}

	// 方針を決めているハンドラーの値は残す。エラーページのno-storeが既定値で緩むことを防ぐ。
	if header.Get("Cache-Control") == "" {
		header.Set("Cache-Control", htmlCacheControl)
	}
}

// responseContentType はこの応答が名乗るContent-Typeを返す。
//
// ハンドラーが設定しなかった場合、net/httpがヘッダーの送出時に本文から型を判定して名乗る。
// 同じ条件でこちらも判定するのは、型を設定せずHTMLを書くハンドラーにも方針を届けるためである。
// 判定した値はヘッダーへ書き戻さない。応答が運ぶContent-Typeは引き続きnet/httpが決める。
//
// 型を名乗らないまま先にステータスを送った応答は、本文が届く前にここへ来るため判定できない。
// net/httpはその時点のヘッダーを写し取り、後から足した値を送らないので、
// 本文を待って判定し直しても間に合わない。
func responseContentType(header http.Header, body []byte) string {
	if contentType := header.Get("Content-Type"); contentType != "" {
		return contentType
	}

	// エンコード済みの本文はnet/httpも判定しないため、この応答は型を名乗らない。
	if header.Get("Content-Encoding") != "" {
		return ""
	}

	if len(body) == 0 {
		return ""
	}

	return http.DetectContentType(body)
}

// varyHasField はVaryに指定のフィールド名が既に列挙されているかを返す。
//
// 複数行のVaryは1つのリストとして解釈されるため、各行をカンマで割ってから比べる。
// フィールド名はHTTPヘッダーの名前であり、大文字小文字を区別しない。
func varyHasField(header http.Header, field string) bool {
	for _, value := range header.Values("Vary") {
		for listed := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(listed), field) {
				return true
			}
		}
	}

	return false
}

// WriteHeader は最終レスポンスのステータスを送出する前に方針を確定する。
func (w *htmlCacheWriter) WriteHeader(status int) {
	// 情報応答の後もContent-Typeは変更できるため、最終レスポンスまで判定を待つ。
	// 101はプロトコルを切り替えてHTTPの応答を終えるため、ここでは最終レスポンスとして扱う。
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}

	w.decide(nil)

	w.ResponseWriter.WriteHeader(status)
}

// Write はステータスを設定せず本文だけを書くハンドラーも覆う。
// templのRenderはこの形で書き出すため、WriteHeaderだけを捕まえるとページ本体に方針が付かない。
func (w *htmlCacheWriter) Write(b []byte) (int, error) {
	w.decide(b)

	return w.ResponseWriter.Write(b)
}

// FlushError はヘッダーが送信される前に方針を確定し、下層のWriterへFlushを委譲する。
// 最初のFlushは暗黙の200を送るため、本文を書いていなくてもここで判定する必要がある。
func (w *htmlCacheWriter) FlushError() error {
	w.decide(nil)

	return http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap は下層のResponseWriterを返す。
// http.ResponseController が、サーバー自身のResponseWriterが実装する追加のインターフェース
// (Flusher・Hijacker・デッドラインの設定など) へ到達するための手段である。
// このラッパーは全ルートを覆うため、これが無いとそれらがどこでも利かなくなる。
func (w *htmlCacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// ReadFrom は本文の書き出しを下層のResponseWriterに任せる。
//
// 下層が io.ReaderFrom を実装していても、インターフェースの埋め込みでは昇格せず io.Copy から見えなくなる。
// net/httpのResponseWriterはそこでsendfileを使い、静的アセットの配信がこの判定を通るため、
// 橋渡ししないとファイルの内容をバッファ経由でコピーすることになる。
//
// 本文はそのまま下層へ渡すため判定には使えない。この経路を通るのは自分で型を名乗る
// http.FileServer の配信であり、見分けるのに本文は要らない。
func (w *htmlCacheWriter) ReadFrom(r io.Reader) (int64, error) {
	w.decide(nil)

	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	return io.Copy(w.ResponseWriter, r)
}
