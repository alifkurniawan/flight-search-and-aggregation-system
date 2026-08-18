package sorts

import (
	"app/internal/models"
	"time"
)

// Weights reflect passenger priority, from most to least important:
// price > number of stops > departure-time convenience > total duration.
// These are a judgment call (the assignment leaves "convenience" undefined)
// -- documented here so the reasoning is explicit, not just the numbers.
const (
	weightPrice     = 0.40
	weightStops     = 0.30
	weightDeparture = 0.20
	weightDuration  = 0.10
)

// ApplyBestValueScores computes and sets BestValueScore (0-100, higher is
// better) on every flight in place. Price, stops, and duration are
// normalized against the min/max observed IN THIS RESULT SET -- so the
// score is always relative to what's actually on offer for this specific
// search, not some fixed global scale.
func ApplyBestValueScores(flights []models.Flight) {
	if len(flights) == 0 {
		return
	}

	minPrice, maxPrice := flights[0].Price.Amount, flights[0].Price.Amount
	minDur, maxDur := flights[0].Duration.TotalMinutes, flights[0].Duration.TotalMinutes
	minStops, maxStops := flights[0].Stops, flights[0].Stops

	for _, f := range flights {
		if f.Price.Amount < minPrice {
			minPrice = f.Price.Amount
		}
		if f.Price.Amount > maxPrice {
			maxPrice = f.Price.Amount
		}
		if f.Duration.TotalMinutes < minDur {
			minDur = f.Duration.TotalMinutes
		}
		if f.Duration.TotalMinutes > maxDur {
			maxDur = f.Duration.TotalMinutes
		}
		if f.Stops < minStops {
			minStops = f.Stops
		}
		if f.Stops > maxStops {
			maxStops = f.Stops
		}
	}

	for i := range flights {
		priceScore := normalizeInverseInt64(flights[i].Price.Amount, minPrice, maxPrice)
		durationScore := normalizeInverseInt(flights[i].Duration.TotalMinutes, minDur, maxDur)
		stopsScore := normalizeInverseInt(flights[i].Stops, minStops, maxStops)
		departureScore := departureConvenienceScore(flights[i].Departure.DateTime)

		score := weightPrice*priceScore +
			weightStops*stopsScore +
			weightDeparture*departureScore +
			weightDuration*durationScore

		flights[i].BestValueScore = round2(score * 100)
	}
}

// departureConvenienceScore scores how convenient a departure time is,
// based on the LOCAL wall-clock hour at the origin airport. It reads the
// hour straight off the parsed RFC3339 string WITHOUT converting to UTC
// first -- "red-eye" is inherently a local-time concept (an 11pm departure
// is inconvenient in Jakarta regardless of what time that is in UTC), and
// every provider adapter already preserves the original local offset in
// this field (see internal/providers), so this is safe.
func departureConvenienceScore(rfc3339 string) float64 {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return 0.5 // unknown/unparseable -> neutral, don't let bad data tank the score
	}

	hour := t.Hour()
	switch {
	case hour >= 9 && hour < 21:
		return 1.0 // ideal daytime window
	case hour >= 6 && hour < 9:
		return 0.8 // early morning, still reasonable
	case hour >= 21 && hour < 23:
		return 0.5 // late evening
	default:
		return 0.2 // 23:00-06:00, red-eye
	}
}

func normalizeInverseInt(value, min, max int) float64 {
	if max == min {
		return 1.0
	}
	return 1.0 - float64(value-min)/float64(max-min)
}

func normalizeInverseInt64(value, min, max int64) float64 {
	if max == min {
		return 1.0
	}
	return 1.0 - float64(value-min)/float64(max-min)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
