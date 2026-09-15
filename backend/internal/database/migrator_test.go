package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	dbmigrations "ai-interview-platform/migrations"
)

func TestEmbedSource_ListsAllUpMigrations(t *testing.T) {
	src := NewEmbedSource(dbmigrations.FS)
	migs, err := src.ListUp()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(migs) < 10 {
		t.Fatalf("expected >=10 embedded up migrations, got %d", len(migs))
	}
	// 版本升序、从 0001 开始、SQL 非空
	for i, m := range migs {
		if m.Version != int64(i+1) {
			t.Fatalf("migrations must be sequential 1..n, got version %d at index %d", m.Version, i)
		}
		if strings.TrimSpace(m.SQL) == "" {
			t.Fatalf("migration %s is empty", m.Filename)
		}
		if !strings.HasSuffix(m.Filename, ".up.sql") {
			t.Fatalf("only up files allowed, got %s", m.Filename)
		}
	}
	// 内嵌 FS 同时含 down 文件，但不应出现在列表中
	for _, m := range migs {
		if strings.Contains(m.Filename, ".down.") {
			t.Fatalf("down migration leaked: %s", m.Filename)
		}
	}
}

func TestParseMigrationFile(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
		err  bool
	}{
		{"0001_create_users_table.up.sql", true, false},
		{"0010_create_answer_audios_table.up.sql", true, false},
		{"0001_create_users_table.down.sql", false, false},
		{"notes.txt", false, false},
	}
	for _, tc := range cases {
		mig, ok, err := parseMigrationFile(tc.name, func(string) ([]byte, error) { return []byte("SELECT 1"), nil })
		if tc.err && err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
		if ok != tc.ok {
			t.Errorf("%s: ok=%v want %v (err=%v)", tc.name, ok, tc.ok, err)
		}
		if tc.ok && mig.Version == 0 {
			t.Errorf("%s: version not parsed", tc.name)
		}
	}
}

func TestSortAndValidate_DuplicateRejected(t *testing.T) {
	dup := []Migration{
		{Version: 2, Filename: "0002_b.up.sql"},
		{Version: 1, Filename: "0001_a.up.sql"},
		{Version: 2, Filename: "0002_c.up.sql"},
	}
	if _, err := sortAndValidate(dup); err == nil ||
		!strings.Contains(err.Error(), "duplicate migration version") {
		t.Fatalf("expected duplicate error, got %v", err)
	}

	ok := []Migration{
		{Version: 3, Filename: "0003.up.sql"},
		{Version: 1, Filename: "0001.up.sql"},
	}
	sorted, err := sortAndValidate(ok)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if sorted[0].Version != 1 || sorted[1].Version != 3 {
		t.Fatalf("not sorted: %+v", sorted)
	}
}

func TestDirSource(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0002_second.up.sql":  "CREATE TABLE b();",
		"0001_first.up.sql":   "CREATE TABLE a();",
		"0001_first.down.sql": "DROP TABLE a;",
		"README.md":           "ignore me",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	migs, err := NewDirSource(dir).ListUp()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(migs) != 2 || migs[0].Version != 1 || migs[1].Version != 2 {
		t.Fatalf("dir source = %+v", migs)
	}
	if migs[0].SQL != "CREATE TABLE a();" {
		t.Fatalf("sql not read: %q", migs[0].SQL)
	}

	// 不存在的目录返回错误
	if _, err := NewDirSource(filepath.Join(dir, "nope")).ListUp(); err == nil {
		t.Fatal("missing dir must error")
	}
}

func TestEmbedSource_NonRecursiveAndIgnoresNonSQL(t *testing.T) {
	mem := fstest.MapFS{
		"0001_a.up.sql":     {Data: []byte("SELECT 1")},
		"0001_a.down.sql":   {Data: []byte("DROP")},
		"notes.txt":         {Data: []byte("hello")},
		"sub/0002_x.up.sql": {Data: []byte("SELECT 2")}, // 子目录不递归扫描
	}
	migs, err := NewEmbedSource(mem).ListUp()
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 1 || migs[0].Filename != "0001_a.up.sql" {
		t.Fatalf("want only root up migration, got %+v", migs)
	}
}
