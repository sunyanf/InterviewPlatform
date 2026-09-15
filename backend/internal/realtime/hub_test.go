package realtime

import (
	"testing"
)

// newHubTestClient 构造不持有真实连接的 Client（仅用于 Hub 簿记测试）
func newHubTestClient(id string) *Client {
	return &Client{
		sessionID: id,
		send:      make(chan []byte, 8),
		shutdown:  make(chan struct{}),
		closed:    make(chan struct{}),
	}
}

func TestHub_ReplaceReturnsOldClient(t *testing.T) {
	h := &Hub{sessionID: "s1"}
	a := newHubTestClient("s1")
	b := newHubTestClient("s1")

	if old := h.replace(a); old != nil {
		t.Fatalf("first replace should have no old client, got %v", old)
	}
	if old := h.replace(b); old != a {
		t.Fatal("second replace must return previous client")
	}
}

func TestHub_ClearOnlyClearsCurrentClient(t *testing.T) {
	h := &Hub{sessionID: "s1"}
	a := newHubTestClient("s1")
	b := newHubTestClient("s1")
	h.replace(a)
	h.replace(b)

	// 旧连接延迟退出时的 clear 不能误删新连接
	h.clear(a)
	if got := h.replace(nil); got != b {
		// replace(nil) 返回当前 client 并置空
		t.Fatal("old client clear must not evict new client")
	}
}

func TestHubManager_AcquireSameHub(t *testing.T) {
	m := NewHubManager()
	h1 := m.acquire("s1")
	h2 := m.acquire("s1")
	if h1 != h2 {
		t.Fatal("same session must return same hub")
	}
	if m.acquire("s2") == h1 {
		t.Fatal("different sessions must get different hubs")
	}
}

func TestHubManager_ReleaseRecyclesEmptyHub(t *testing.T) {
	m := NewHubManager()
	h := m.acquire("s1")
	m.release("s1", h)
	if m.acquire("s1") == h {
		t.Fatal("empty hub should be recycled")
	}
}

func TestHubManager_ReleaseKeepsHubWithClient(t *testing.T) {
	m := NewHubManager()
	h := m.acquire("s1")
	h.replace(newHubTestClient("s1"))
	m.release("s1", h)
	if m.acquire("s1") != h {
		t.Fatal("hub with active client must not be recycled")
	}
}

func TestHubManager_ReleaseKeepsHubAfterReplacement(t *testing.T) {
	m := NewHubManager()
	h := m.acquire("s1")
	a := newHubTestClient("s1")
	h.replace(a)

	// 模拟旧连接 release 与新连接顶替交错：新连接已在 hub 上
	b := newHubTestClient("s1")
	h.replace(b)
	h.clear(a)
	m.release("s1", h)

	if m.acquire("s1") != h {
		t.Fatal("hub must survive release from replaced old connection")
	}
}
