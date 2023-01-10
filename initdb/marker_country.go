package initdb

import (
	"fmt"
	"log"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/service"
)

func AppendMarkerCountryColumn() error {
	var markers []dbmodel.Marker

	query := database.Connection
	query = query.Where("country IS NULL")

	if err := query.Find(&markers).Error; err != nil {
		return err
	}

	if len(markers) != 0 {
		log.Println("Detected markers without country info, updating...")
	}

	for _, marker := range markers {
		tomtomResp, err := service.GetReverseGeocode(marker.Latitude, marker.Longitude)

		if err == nil {
			if len(tomtomResp.Addresses) != 0 {
				marker.Country = tomtomResp.Addresses[0].Address.Country
				marker.CountryCode = tomtomResp.Addresses[0].Address.CountryCode
				marker.CountryPart = tomtomResp.Addresses[0].Address.LocalName

				marker.Update(database.Connection)
			}
		} else {
			fmt.Println(err)
		}
	}

	return nil
}
