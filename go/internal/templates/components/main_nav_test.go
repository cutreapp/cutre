package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
)

// TestMainNav は、メインメニューがホーム・リスト・交換・メッセージ・マイページへのリンクを持ち、表示中のページを指すリンクに aria-current="page" を、
// 項目から辿った画面を表示しているときはその項目に aria-current="true" を付けることを検証する。
func TestMainNav(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   components.MainNavData
		want   []string
		absent []string
	}{
		{
			name: "ホームを表示中",
			data: components.MainNavData{Atname: "cutre_user", Current: components.MainNavHome, CurrentPath: templates.HomePath},
			want: []string{
				`<nav aria-label="メインメニュー"`,
				`href="/home"`,
				`href="/list"`,
				`href="/trades"`,
				`href="/messages"`,
				`href="/@cutre_user"`,
				"ホーム",
				"リスト",
				"交換",
				"メッセージ",
				"マイページ",
			},
			absent: []string{`aria-current="true"`, "max-md:hidden"},
		},
		{
			name:   "マイページを表示中",
			data:   components.MainNavData{Atname: "cutre_user", Current: components.MainNavMyPage, CurrentPath: "/@cutre_user"},
			want:   []string{`href="/@cutre_user" class=`},
			absent: []string{`aria-current="true"`},
		},
		{
			name:   "リストを表示中",
			data:   components.MainNavData{Atname: "cutre_user", Current: components.MainNavList, CurrentPath: templates.ListPath},
			want:   []string{`href="/list" class=`, `aria-current="page"`},
			absent: []string{`aria-current="true"`},
		},
		{
			name:   "リストから辿った画面を表示中",
			data:   components.MainNavData{Atname: "cutre_user", Current: components.MainNavList, CurrentPath: templates.EditItemPath("0199a1b2-0000-7000-8000-000000000000")},
			want:   []string{`aria-current="true"`},
			absent: []string{`aria-current="page"`},
		},
		{
			name:   "交換から辿った画面を表示中",
			data:   components.MainNavData{Atname: "cutre_user", Current: components.MainNavTrade, CurrentPath: templates.MatchesPath},
			want:   []string{`aria-current="true"`},
			absent: []string{`aria-current="page"`},
		},
		{
			name:   "メッセージから辿った画面を表示中",
			data:   components.MainNavData{Atname: "cutre_user", Current: components.MainNavMessage, CurrentPath: templates.TradeMessagesPath("0199a1b2-0000-7000-8000-000000000000")},
			want:   []string{`href="/messages" class=`, `aria-current="true"`},
			absent: []string{`aria-current="page"`},
		},
		{
			name:   "下のメニューを隠す",
			data:   components.MainNavData{Atname: "cutre_user", Current: components.MainNavMessage, CurrentPath: templates.TradeMessagesPath("0199a1b2-0000-7000-8000-000000000000"), BottomHidden: true},
			want:   []string{`md:pb-0 max-md:hidden"`, `aria-current="true"`},
			absent: []string{`aria-current="page"`},
		},
		{
			name:   "マイページから辿った画面を表示中",
			data:   components.MainNavData{Atname: "cutre_user", Current: components.MainNavMyPage, CurrentPath: templates.SettingsInvitationPath},
			want:   []string{`aria-current="true"`},
			absent: []string{`aria-current="page"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := render(t, components.MainNav(tt.data))

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(got, absent) {
					t.Errorf("出力に %q が含まれている\n出力: %s", absent, got)
				}
			}
			// 選択中にする項目は1つだけで、ほかの項目には aria-current を付けない。
			if count := strings.Count(got, "aria-current="); count != 1 {
				t.Errorf("aria-current の数 = %d、期待値 = 1\n出力: %s", count, got)
			}
			// ログイン後のページでは「Cutre」の文字の見出しを出さない。
			if strings.Contains(got, "Cutre") {
				t.Errorf("出力に「Cutre」の文字が含まれている\n出力: %s", got)
			}
		})
	}
}

// TestMainNav_CurrentLink は、aria-current を表示中の項目のリンクに付けることを検証する。
func TestMainNav_CurrentLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     components.MainNavData
		wantHref string
	}{
		{
			name:     "ホーム",
			data:     components.MainNavData{Atname: "cutre_user", Current: components.MainNavHome, CurrentPath: templates.HomePath},
			wantHref: `href="/home"`,
		},
		{
			name:     "交換",
			data:     components.MainNavData{Atname: "cutre_user", Current: components.MainNavTrade, CurrentPath: templates.TradesPath},
			wantHref: `href="/trades"`,
		},
		{
			name:     "メッセージ",
			data:     components.MainNavData{Atname: "cutre_user", Current: components.MainNavMessage, CurrentPath: templates.MessagesPath},
			wantHref: `href="/messages"`,
		},
		{
			name:     "マイページ",
			data:     components.MainNavData{Atname: "cutre_user", Current: components.MainNavMyPage, CurrentPath: "/@cutre_user"},
			wantHref: `href="/@cutre_user"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := render(t, components.MainNav(tt.data))

			// aria-current はリンクの開始タグの中に出る。
			start := strings.Index(got, tt.wantHref)
			if start < 0 {
				t.Fatalf("出力に %q が含まれていない\n出力: %s", tt.wantHref, got)
			}
			tag := got[start : start+strings.Index(got[start:], ">")]
			if !strings.Contains(tag, `aria-current="page"`) {
				t.Errorf("%s のリンクに aria-current=\"page\" が付いていない\nタグ: %s", tt.wantHref, tag)
			}
		})
	}
}

// TestMainNav_AwaitingTradeBadge は、返事や確認を待っている交換があるときだけ、交換の項目のアイコンに数字を重ね、
// 数を含めた名前をリンクに付けることを検証する。
func TestMainNav_AwaitingTradeBadge(t *testing.T) {
	t.Parallel()

	data := components.MainNavData{Atname: "cutre_user", Current: components.MainNavHome, CurrentPath: templates.HomePath}

	got := renderWithContext(t, templates.WithMainNavBadges(context.Background(), templates.MainNavBadges{AwaitingTradeCount: 12}), components.MainNav(data))
	for _, want := range []string{
		`href="/trades" class=`,
		`aria-label="交換 (対応待ち 12件)"`,
		`<span aria-hidden="true" class="badge absolute -top-2 -right-3.5" data-variant="count">12</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}

	got = render(t, components.MainNav(data))
	for _, absent := range []string{`aria-label="交換`, `data-variant="count"`} {
		if strings.Contains(got, absent) {
			t.Errorf("待っている交換が無いのに、出力に %q が含まれている\n出力: %s", absent, got)
		}
	}
}

// TestMainNav_UnreadMessageBadge は、未読のメッセージがあるときだけ、メッセージの項目のアイコンに数字を重ね、
// 数を含めた名前をリンクに付けることを検証する。
func TestMainNav_UnreadMessageBadge(t *testing.T) {
	t.Parallel()

	data := components.MainNavData{Atname: "cutre_user", Current: components.MainNavHome, CurrentPath: templates.HomePath}

	got := renderWithContext(t, templates.WithMainNavBadges(context.Background(), templates.MainNavBadges{UnreadMessageCount: 3}), components.MainNav(data))
	for _, want := range []string{
		`href="/messages" class=`,
		`aria-label="メッセージ (未読 3件)"`,
		`<span aria-hidden="true" class="badge absolute -top-2 -right-3.5" data-variant="count">3</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}
	if strings.Contains(got, `aria-label="交換`) {
		t.Errorf("待っている交換が無いのに、交換の項目に数を含めた名前を付けた\n出力: %s", got)
	}

	got = render(t, components.MainNav(data))
	if strings.Contains(got, `aria-label="メッセージ`) {
		t.Errorf("未読のメッセージが無いのに、数を含めた名前を付けた\n出力: %s", got)
	}
}

// renderWithContext は ctx に日本語のロケールを載せてコンポーネントを描画する。
func renderWithContext(t *testing.T, ctx context.Context, component templ.Component) string {
	t.Helper()

	var buf strings.Builder
	if err := component.Render(i18n.SetLocale(ctx, i18n.LangJa), &buf); err != nil {
		t.Fatalf("描画のエラー = %v", err)
	}
	return buf.String()
}
