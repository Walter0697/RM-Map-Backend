package seed

import (
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/service"
	"mapmarker/backend/utils"
	"strconv"
	"time"

	"syreclabs.com/go/faker"
)

func SeedMarkers() {
	log.Println("seeding markers...")

	types := []string{}
	eventtypes, err := service.GetAllEventType()
	if err != nil {
		panic(err)
	}
	for _, et := range eventtypes {
		types = append(types, et.Value)
	}

	var user dbmodel.User
	user.ID = config.Data.Seed.CreateUserId
	if err := user.GetUserById(); err != nil {
		panic(err)
	}

	for i := 0; i < config.Data.Seed.MarkerNums; i++ {
		marker := MarkerFactory(types, user, i)
		if err := marker.Create(); err != nil {
			panic(err)
		}
	}
}

func MarkerFactory(eventtypes []string, user dbmodel.User, index int) dbmodel.Marker {
	now := time.Now()

	var marker dbmodel.Marker
	marker.Label = faker.Name().Title()
	marker.Latitude = RandomOffSet(config.Data.Seed.CenterLatitude, config.Data.Seed.CenterOffset)
	marker.Longitude = RandomOffSet(config.Data.Seed.CenterLongitude, config.Data.Seed.CenterOffset)
	marker.Address = faker.Address().String()
	if WithChance(1, 4) {
		imageLink := faker.Avatar().Url("jpg", 300, 300)
		filename, err := UploadMarkerImage(index, imageLink, "jpg")
		if err != nil {
			panic(err)
		}
		marker.ImageLink = filename
	}
	marker.Type = RandomStringInList(eventtypes)
	marker.Description = faker.Hacker().SaySomethingSmart()
	marker.Link = faker.Internet().Url()
	marker.EstimateTime = RandomStringInList([]string{"", "short", "medium", "long"})
	marker.Price = RandomStringInList([]string{"", "free", "cheap", "middle", "expensive"})
	if WithChance(1, 5) {
		tendaysbefore := now.AddDate(0, 0, -10)
		marker.FromTime = &tendaysbefore

		randday := RandomInteger(2, 10)
		randdaysafter := now.AddDate(0, 0, randday)
		marker.ToTime = &randdaysafter
	}
	marker.RelationId = config.Data.Seed.MarkerRelationId
	marker.Status = ""

	marker.CreatedBy = &user
	marker.UpdatedBy = &user

	marker.IsFavourite = RandomBool()

	return marker
}

func UploadMarkerImage(index int, url string, ext string) (string, error) {
	now := time.Now()

	filename := constant.MarkerPreviewPath + "DUMMY" + now.Format("2006-01-02T15-04-05-07-00") + strconv.Itoa(index) + "." + ext
	filepath := constant.BasePath + filename
	if err := utils.SaveImageFromURL(filepath, url); err != nil {
		return "", err
	}

	return filename, nil
}
