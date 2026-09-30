package components_test

import (
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestAdminMasterList は、行を編集の画面へのリンクにしてアーカイブしたものにバッジを添え、
// 見出しと作成の画面へのリンクを置くことを検証する。
func TestAdminMasterList(t *testing.T) {
	t.Parallel()

	got := render(t, components.AdminMasterList(components.AdminMasterListData{
		ID:           "list-heading",
		Heading:      "カテゴリー",
		Rows:         []viewmodel.AdminMasterRow{{ID: "a", Name: "A賞"}, {ID: "b", Name: "B賞", Archived: true}},
		EditPath:     func(id string) string { return "/edit/" + id },
		NewPath:      "/new",
		NewLabel:     "カテゴリーを作成",
		EmptyMessage: "カテゴリーはまだありません",
	}))

	for _, want := range []string{
		`<section class="flex flex-col gap-2" aria-labelledby="list-heading">`,
		`href="/edit/a"`,
		`href="/edit/b"`,
		`<span class="badge" data-variant="warning">アーカイブ</span>`,
		`href="/new"`,
		"カテゴリーを作成",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}
	if strings.Count(got, `data-variant="warning"`) != 1 || strings.Contains(got, "カテゴリーはまだありません") {
		t.Errorf("バッジはアーカイブした行にだけ付け、行があるときは空の文言を出さないことを期待\n出力: %s", got)
	}
}

// TestAdminMasterList_Empty は、行が1つも無いときに空の文言を出すことを検証する。
func TestAdminMasterList_Empty(t *testing.T) {
	t.Parallel()

	got := render(t, components.AdminMasterList(components.AdminMasterListData{
		ID:           "list-heading",
		Heading:      "カテゴリー",
		EditPath:     func(id string) string { return "/edit/" + id },
		NewPath:      "/new",
		NewLabel:     "カテゴリーを作成",
		EmptyMessage: "カテゴリーはまだありません",
	}))

	if !strings.Contains(got, "カテゴリーはまだありません") || strings.Contains(got, "<ul") {
		t.Errorf("空の文言だけを出すことを期待\n出力: %s", got)
	}
}
