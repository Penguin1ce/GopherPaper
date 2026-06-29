package translate

import (
	"sync"
	"testing"
	"time"

	"GopherPaper/pkg/constant"
)

func TestTranslationCacheScopedByUser(t *testing.T) {
	resultCache = syncMapForTest()
	now := time.Now()
	storeTranslation("u1", "hello", "你好", now)

	if got, ok := cachedTranslation("u1", "hello", now.Add(time.Minute)); !ok || got != "你好" {
		t.Fatalf("same user/text should hit cache, got %q hit=%v", got, ok)
	}
	if got, ok := cachedTranslation("u2", "hello", now.Add(time.Minute)); ok || got != "" {
		t.Fatalf("different user should miss cache, got %q hit=%v", got, ok)
	}
}

func TestTranslationCacheExpires(t *testing.T) {
	resultCache = syncMapForTest()
	now := time.Now()
	storeTranslation("u1", "hello", "你好", now)

	if got, ok := cachedTranslation("u1", "hello", now.Add(constant.TranslateCacheTTL+time.Second)); ok || got != "" {
		t.Fatalf("expired cache should miss, got %q hit=%v", got, ok)
	}
}

func TestTranslationCacheSkipsBlankTranslation(t *testing.T) {
	resultCache = syncMapForTest()
	now := time.Now()
	storeTranslation("u1", "hello", "   ", now)

	if got, ok := cachedTranslation("u1", "hello", now); ok || got != "" {
		t.Fatalf("blank translation should not be cached, got %q hit=%v", got, ok)
	}
}

func syncMapForTest() sync.Map {
	return sync.Map{}
}
