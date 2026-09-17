package download

import "testing"

func TestRequiredSplitDiskBytesIncludesOutputAndReserve(t *testing.T) {
	const sourceSize = int64(5 * 1024 * 1024 * 1024)
	want := sourceSize + splitDiskReserve
	if got := requiredSplitDiskBytes(sourceSize); got != want {
		t.Fatalf("requiredSplitDiskBytes() = %d, want %d", got, want)
	}
}
