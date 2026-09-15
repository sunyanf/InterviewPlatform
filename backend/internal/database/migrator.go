package database

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationFileRE 迁移文件名规则：0001_create_users_table.up.sql
var migrationFileRE = regexp.MustCompile(`^(\d{4,})_.+\.up\.sql$`)

// Migration 一个 up 迁移
type Migration struct {
	Version  int64
	Filename string
	SQL      string
}

// Source 迁移来源（内嵌 FS 或本地目录）
type Source interface {
	ListUp() ([]Migration, error)
}

// Migrator 数据库迁移器：每个版本仅应用一次，记录于 schema_migrations。
// 现有 up 迁移均幂等（IF NOT EXISTS / ON CONFLICT），
// 因此在"已手工建表但无版本记录"的库上首次运行也安全：重跑后补记版本。
type Migrator struct {
	pool *pgxpool.Pool
	src  Source
}

// NewMigrator 创建迁移器
func NewMigrator(pool *pgxpool.Pool, src Source) *Migrator {
	return &Migrator{pool: pool, src: src}
}

// Status 迁移状态
type Status struct {
	// Current 当前已应用的最大版本（无迁移时为 0）
	Current int64
	// Applied 已应用版本集合
	Applied map[int64]bool
	// Pending 待应用迁移（按版本升序）
	Pending []Migration
}

// createTableDDL 版本记录表（迁移器自身的 bootstrap DDL，幂等）
const createTableDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version    BIGINT PRIMARY KEY,
    filename   TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

// Up 应用全部待执行迁移；每个迁移在独立事务内执行并记录版本。
// 返回本次新应用的迁移列表。
func (m *Migrator) Up(ctx context.Context) ([]Migration, error) {
	if _, err := m.pool.Exec(ctx, createTableDDL); err != nil {
		return nil, fmt.Errorf("ensure schema_migrations: %w", err)
	}

	status, err := m.Status(ctx)
	if err != nil {
		return nil, err
	}
	if len(status.Pending) == 0 {
		return nil, nil
	}

	applied := make([]Migration, 0, len(status.Pending))
	for _, mig := range status.Pending {
		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("begin tx for %s: %w", mig.Filename, err)
		}
		if _, err := tx.Exec(ctx, mig.SQL); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("apply %s: %w", mig.Filename, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, filename) VALUES ($1, $2)`,
			mig.Version, mig.Filename); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("record %s: %w", mig.Filename, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("commit %s: %w", mig.Filename, err)
		}
		applied = append(applied, mig)
	}
	return applied, nil
}

// Status 查询已应用版本与待执行迁移
func (m *Migrator) Status(ctx context.Context) (*Status, error) {
	if _, err := m.pool.Exec(ctx, createTableDDL); err != nil {
		return nil, fmt.Errorf("ensure schema_migrations: %w", err)
	}

	rows, err := m.pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	var current int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
		if v > current {
			current = v
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	all, err := m.src.ListUp()
	if err != nil {
		return nil, err
	}
	pending := make([]Migration, 0)
	for _, mig := range all {
		if !applied[mig.Version] {
			pending = append(pending, mig)
		}
	}
	return &Status{Current: current, Applied: applied, Pending: pending}, nil
}

// ---------- Sources ----------

// embedSource 内嵌 FS 来源
type embedSource struct{ fsys fs.FS }

// NewEmbedSource 从 go:embed FS 创建迁移来源
func NewEmbedSource(fsys fs.FS) Source {
	return &embedSource{fsys: fsys}
}

func (s *embedSource) ListUp() ([]Migration, error) {
	entries, err := fs.ReadDir(s.fsys, ".")
	if err != nil {
		return nil, err
	}
	migs := make([]Migration, 0, len(entries))
	for _, d := range entries {
		if d.IsDir() || filepath.Ext(d.Name()) != ".sql" {
			continue
		}
		mig, ok, err := parseMigrationFile(d.Name(), func(n string) ([]byte, error) {
			return fs.ReadFile(s.fsys, n)
		})
		if err != nil {
			return nil, err
		}
		if ok {
			migs = append(migs, mig)
		}
	}
	return sortAndValidate(migs)
}

// dirSource 本地目录来源（本地开发/运维自定义迁移目录）
type dirSource struct{ dir string }

// NewDirSource 从文件系统目录创建迁移来源
func NewDirSource(dir string) Source {
	return &dirSource{dir: dir}
}

func (s *dirSource) ListUp() ([]Migration, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", s.dir, err)
	}
	migs := make([]Migration, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		mig, ok, err := parseMigrationFile(e.Name(), func(n string) ([]byte, error) {
			return os.ReadFile(filepath.Join(s.dir, n))
		})
		if err != nil {
			return nil, err
		}
		if ok {
			migs = append(migs, mig)
		}
	}
	return sortAndValidate(migs)
}

// parseMigrationFile 解析 up 迁移文件；非 up 文件（如 down）返回 ok=false
func parseMigrationFile(name string, read func(string) ([]byte, error)) (mig Migration, ok bool, err error) {
	m := migrationFileRE.FindStringSubmatch(name)
	if m == nil {
		return Migration{}, false, nil
	}
	version, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return Migration{}, false, fmt.Errorf("invalid version in %s: %w", name, err)
	}
	content, err := read(name)
	if err != nil {
		return Migration{}, false, fmt.Errorf("read %s: %w", name, err)
	}
	return Migration{Version: version, Filename: name, SQL: string(content)}, true, nil
}

// sortAndValidate 按版本排序并拒绝重复版本号
func sortAndValidate(migs []Migration) ([]Migration, error) {
	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	seen := make(map[int64]string, len(migs))
	for _, mig := range migs {
		if prev, dup := seen[mig.Version]; dup {
			return nil, fmt.Errorf("duplicate migration version %04d: %s and %s", mig.Version, prev, mig.Filename)
		}
		seen[mig.Version] = mig.Filename
	}
	return migs, nil
}
