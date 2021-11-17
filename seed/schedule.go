package seed

import (
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/service"
	"time"

	"syreclabs.com/go/faker"
)

func SeedSchedules() {
	log.Println("seeding schedules...")

	var relation dbmodel.UserRelation
	relation.ID = config.Data.Seed.MarkerRelationId
	if err := relation.GetRelationById(); err != nil {
		panic(err)
	}

	markers, err := service.GetAllActiveMarker([]string{}, relation)
	if err != nil {
		panic(err)
	}

	var user dbmodel.User
	user.ID = config.Data.Seed.CreateUserId
	if err := user.GetUserById(); err != nil {
		panic(err)
	}

	now := time.Now()
	for i := 0; i < config.Data.Seed.ScheduleDays; i++ {
		selected := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		for j := 0; j < 8; j++ {
			selected.Add(time.Hour * time.Duration(1))
			schedule := ScheduleFactory(markers, user, selected, i*j)
			if err := schedule.Create(); err != nil {
				panic(err)
			}
		}
		now = now.AddDate(0, 0, 1)
	}
}

func ScheduleFactory(markers []dbmodel.Marker, user dbmodel.User, selectedDate time.Time, index int) dbmodel.Schedule {
	var schedule dbmodel.Schedule
	schedule.Label = faker.Name().Title()
	schedule.Description = faker.Hacker().SaySomethingSmart()
	marker := RandomMarker(markers)
	schedule.SelectedMarker = &marker
	schedule.SelectedDate = selectedDate

	schedule.RelationId = config.Data.Seed.MarkerRelationId
	schedule.Status = ""

	marker.CreatedBy = &user
	marker.UpdatedBy = &user

	return schedule
}

func RandomMarker(markers []dbmodel.Marker) dbmodel.Marker {
	randIndex := RandomInteger(0, len(markers))
	return markers[randIndex]
}
