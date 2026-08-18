package utils

import "strings"

var airportCities = map[string]string{
	"CGK": "Jakarta",
	"DPS": "Denpasar",
	"SOC": "Surakarta",
	"SUB": "Surabaya",
	"JOG": "Yogyakarta",
	"BDO": "Bandung",
	"UPG": "Makassar",
	"KNO": "Medan",
}

func CityFromAirport(code string) string {
	city, ok := airportCities[strings.ToUpper(strings.TrimSpace(code))]
	if !ok {
		return "-"
	}

	return city
}
