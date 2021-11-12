package seed

import (
	"log"
	"mapmarker/backend/config"
	"math/rand"
)

func SeedDatabase() {
	if config.Data.Seed.EnableSeed {
		SeedMarkers()
	} else {
		log.Println("seeding not enabled in config")
	}
}

func RandomInteger(min int, max int) int {
	return rand.Int()%(max-min) + min
}

func RandomFloat(min float64, max float64) float64 {
	return rand.Float64()*(max-min) + min
}

func RandomStringInList(list []string) string {
	randIndex := RandomInteger(0, len(list))
	return list[randIndex]
}

func RandomOffSet(origin float64, offset float64) float64 {
	offsetValue := RandomFloat(-offset, offset)
	return origin + offsetValue
}

func RandomBool() bool {
	return rand.Int()%2 == 1
}

func WithChance(chance int, outof int) bool {
	result := RandomInteger(0, outof)
	if chance > result {
		return true
	}
	return false
}
