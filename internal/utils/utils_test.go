package utils_test

import (
	"app/internal/utils"
	"testing"
)

func TestCityFromAirport(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{"CGK returns Jakarta", "CGK", "Jakarta"},
		{"DPS returns Denpasar", "DPS", "Denpasar"},
		{"SOC returns Surakarta", "SOC", "Surakarta"},
		{"SUB returns Surabaya", "SUB", "Surabaya"},
		{"JOG returns Yogyakarta", "JOG", "Yogyakarta"},
		{"BDO returns Bandung", "BDO", "Bandung"},
		{"UPG returns Makassar", "UPG", "Makassar"},
		{"KNO returns Medan", "KNO", "Medan"},
		{"lowercase cgk still works", "cgk", "Jakarta"},
		{"with spaces still works", " DPS ", "Denpasar"},
		{"unknown code returns dash", "XXX", "-"},
		{"empty code returns dash", "", "-"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := utils.CityFromAirport(tc.code)
			if got != tc.expected {
				t.Errorf("CityFromAirport(%q) = %q, want %q", tc.code, got, tc.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name         string
		totalMinutes int
		expected     string
	}{
		{"zero minutes", 0, "0h 0m"},
		{"negative minutes clamps to zero", -10, "0h 0m"},
		{"30 minutes", 30, "0h 30m"},
		{"60 minutes is 1h", 60, "1h 0m"},
		{"90 minutes is 1h 30m", 90, "1h 30m"},
		{"105 minutes is 1h 45m", 105, "1h 45m"},
		{"120 minutes is 2h", 120, "2h 0m"},
		{"230 minutes is 3h 50m", 230, "3h 50m"},
		{"1 minute", 1, "0h 1m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := utils.FormatDuration(tc.totalMinutes)
			if got != tc.expected {
				t.Errorf("FormatDuration(%d) = %q, want %q", tc.totalMinutes, got, tc.expected)
			}
		})
	}
}

func TestFormatIDR(t *testing.T) {
	cases := []struct {
		amount int64
		want   string
	}{
		{0, "Rp0"},
		{500, "Rp500"},
		{485000, "Rp485.000"},
		{1500000, "Rp1.500.000"},
		{-750000, "-Rp750.000"},
	}
	for _, c := range cases {
		if got := utils.FormatIDR(c.amount); got != c.want {
			t.Errorf("FormatIDR(%d) = %q, want %q", c.amount, got, c.want)
		}
	}
}
