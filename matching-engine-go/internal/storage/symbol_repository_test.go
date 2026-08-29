package storage

import (
	"bist-matching-engine/internal/domain"
	"testing"
)

func TestPostgresStore_InsertAndGetSymbol(t *testing.T) {
	store, ctx := newTestStore(t)

	symbol, err := domain.NewSymbol("TERA", 1)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	err = store.InsertSymbol(ctx, symbol)
	if err != nil {
		t.Fatalf("InsertSymbol failed: %v", err)
	}
	t.Cleanup(func() {
		store.pool.Exec(ctx, "DELETE FROM symbols WHERE code = $1", symbol.Code)
	})

	got, err := store.GetSymbol(ctx, symbol.Code)
	if err != nil {
		t.Fatalf("GetSymbol failed: %v", err)
	}
	if symbol.Code != got.Code {
		t.Fatalf("Expected returned code to be same with symbol code; expected: %s, got: %s", symbol.Code, got.Code)
	}

}
