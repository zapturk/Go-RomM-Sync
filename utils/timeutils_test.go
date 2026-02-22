package utils

import (
	"testing"
	"time"
)

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "RFC3339",
			input: "2023-10-27T10:00:00Z",
			want:  time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339 with Offset",
			input: "2023-10-27T10:00:00+02:00",
			want:  time.Date(2023, 10, 27, 10, 0, 0, 0, time.FixedZone("", 2*3600)),
		},
		{
			name:  "RFC3339Nano",
			input: "2023-10-27T10:00:00.123456789Z",
			want:  time.Date(2023, 10, 27, 10, 0, 0, 123456789, time.UTC),
		},
		{
			name:  "Common DB Style",
			input: "2023-10-27 10:00:00",
			want:  time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
		},
		{
			name:  "ISO 8601 variant",
			input: "2023-10-27T10:00:00",
			want:  time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
		},
		{
			name:    "Invalid format",
			input:   "not-a-date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}
