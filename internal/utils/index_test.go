package utils

import "testing"

func TestOffsetToIndex(t *testing.T) {
	tests := []struct {
		name   string
		offset uint
		rawLen int
		want   int
	}{
		{
			name:   "offset within raw length",
			offset: 3,
			rawLen: 10,
			want:   3,
		},
		{
			name:   "offset beyond raw length",
			offset: 99,
			rawLen: 10,
			want:   10,
		},
		{
			name:   "offset beyond int range",
			offset: ^uint(0),
			rawLen: 10,
			want:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OffsetToIndex(tt.offset, tt.rawLen); got != tt.want {
				t.Fatalf("OffsetToIndex(%d, %d) = %d, want %d", tt.offset, tt.rawLen, got, tt.want)
			}
		})
	}
}
