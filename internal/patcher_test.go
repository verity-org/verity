package internal

import (
	"context"
	"testing"
)

func TestImageExistsReturnsFalseForNonExistent(t *testing.T) {
	ctx := context.Background()
	// Use a reference that will never exist in any registry.
	got := imageExists(ctx, "localhost:0/no-such-repo:no-such-tag")
	if got {
		t.Error("imageExists returned true for a non-existent image, want false")
	}
}
