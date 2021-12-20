package service

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"
	"time"

	"gorm.io/gorm"
)

func CreateSchedule(tx *gorm.DB, input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.Schedule, error) {
	var schedule dbmodel.Schedule

	schedule.Label = input.Label
	schedule.Description = input.Description

	// 29/11/2021 : if marker is permanent, don't put it into scheduled state
	if !marker.Permanent {
		marker.Status = constant.Scheduled
		if err := marker.Update(tx); err != nil {
			return nil, err
		}
	}

	schedule.SelectedMarker = &marker

	selectedTime, err := time.Parse(utils.StandardTime, input.SelectedTime)
	if err != nil {
		return nil, err
	}
	schedule.SelectedDate = selectedTime

	schedule.Relation = relation
	schedule.Status = ""
	schedule.CreatedBy = &user
	schedule.UpdatedBy = &user

	if err := schedule.Create(tx); err != nil {
		return nil, err
	}

	return &schedule, nil
}

func CreateMovieSchedule(tx *gorm.DB, input model.NewMovieSchedule, movie dbmodel.Movie, marker *dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.Schedule, error) {
	var schedule dbmodel.Schedule

	schedule.Label = input.Label
	schedule.Description = input.Description

	if marker != nil {
		if !marker.Permanent {
			marker.Status = constant.Scheduled
			if err := marker.Update(tx); err != nil {
				return nil, err
			}
		}

		schedule.SelectedMarker = marker
	}

	selectedTime, err := time.Parse(utils.StandardTime, input.SelectedTime)
	if err != nil {
		return nil, err
	}
	schedule.SelectedDate = selectedTime

	schedule.SelectedMovie = &movie

	schedule.Relation = relation
	schedule.Status = ""
	schedule.CreatedBy = &user
	schedule.UpdatedBy = &user

	if err := schedule.Create(tx); err != nil {
		return nil, err
	}

	return &schedule, nil
}

func EditSchedule(input model.UpdateSchedule, relation dbmodel.UserRelation, user dbmodel.User) (*dbmodel.Schedule, error) {
	var schedule dbmodel.Schedule

	schedule.ID = uint(input.ID)
	if err := schedule.GetById(database.Connection); err != nil {
		return nil, err
	}

	if schedule.RelationId != relation.ID {
		return nil, &helper.InvalidRelationUpdateError{}
	}

	if input.Label != nil {
		schedule.Label = *input.Label
	}

	if input.Description != nil {
		schedule.Description = *input.Description
	}

	if input.SelectedTime != nil {
		selectedTime, err := time.Parse(utils.StandardTime, *input.SelectedTime)
		if err != nil {
			return nil, err
		}
		schedule.SelectedDate = selectedTime
	}

	schedule.UpdatedBy = &user

	if err := schedule.Update(database.Connection); err != nil {
		return nil, err
	}

	return &schedule, nil
}

func RemoveSchedule(tx *gorm.DB, input model.RemoveModel) error {
	var schedule dbmodel.Schedule

	schedule.ID = uint(input.ID)

	if err := schedule.RemoveById(tx); err != nil {
		return err
	}

	return nil
}

func GetAllSchedule(input model.CurrentTime, requested []string, relation dbmodel.UserRelation) ([]dbmodel.Schedule, error) {
	var schedules []dbmodel.Schedule
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}
	if utils.StringInSlice("marker", requested) {
		query = query.Preload("SelectedMarker")
	}

	query = query.Where("relation_id = ?", relation.ID)

	// filter previous schedules
	now, err := time.Parse(utils.DayOnlyTime, input.Time)
	if err != nil {
		return schedules, err
	}
	query = query.Where("selected_date >= ?", now.Format(time.RFC3339))

	if err := query.Find(&schedules).Error; err != nil {
		return schedules, err
	}

	return schedules, nil
}

func UpdateScheduleStatus(tx *gorm.DB, input model.ScheduleStatusList, relation dbmodel.UserRelation, user dbmodel.User) ([]dbmodel.Schedule, error) {
	var schedules []dbmodel.Schedule

	for _, updateStatus := range input.Ids {
		var schedule dbmodel.Schedule
		schedule.ID = uint(updateStatus.ID)
		if err := schedule.GetById(tx); err != nil {
			return schedules, helper.GetDatabaseError(err)
		}

		if schedule.RelationId != relation.ID {
			return schedules, &helper.InvalidRelationUpdateError{}
		}

		schedule.Status = updateStatus.Status

		// 29/11/2021 : if the marker is permanent. don't change the state
		if !schedule.SelectedMarker.Permanent {
			// update marker based on the current status as well
			if updateStatus.Status == constant.Arrived {
				schedule.SelectedMarker.Status = constant.Arrived

				if err := schedule.SelectedMarker.Update(tx); err != nil {
					return schedules, err
				}
			} else if updateStatus.Status == constant.Cancelled {
				schedule.SelectedMarker.Status = constant.Empty

				if err := schedule.SelectedMarker.Update(tx); err != nil {
					return schedules, err
				}
			} else if updateStatus.Status == constant.Empty {
				schedule.SelectedMarker.Status = constant.Scheduled

				if err := schedule.SelectedMarker.Update(tx); err != nil {
					return schedules, err
				}
			}
		}

		schedule.UpdatedBy = &user

		if err := schedule.Update(tx); err != nil {
			return schedules, err
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// this function is used to get all yesterday schedules
func GetYesterdaySchedules(input model.CurrentTime, requested []string, relation dbmodel.UserRelation) ([]dbmodel.Schedule, error) {
	var schedules []dbmodel.Schedule
	query := database.Connection
	if utils.StringInSlice("yesterday_event.created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("yesterday_event.updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}
	if utils.StringInSlice("yesterday_event.marker", requested) {
		query = query.Preload("SelectedMarker")
	}

	query = query.Where("relation_id = ?", relation.ID)

	// filter only yesterday schedules
	now, err := time.Parse(utils.DayOnlyTime, input.Time)
	if err != nil {
		return schedules, err
	}

	// 05/12 : don't restrict on just yesterday
	//yesterday := now.AddDate(0, 0, -1)
	//start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	query = query.Where("selected_date < ?", end.Format(time.RFC3339))

	if err := query.Find(&schedules).Error; err != nil {
		return schedules, err
	}

	return schedules, nil
}

func GetSchedulesByMarker(markerId uint, requested []string, relation dbmodel.UserRelation) ([]dbmodel.Schedule, error) {
	var schedules []dbmodel.Schedule
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}
	if utils.StringInSlice("marker", requested) {
		query = query.Preload("SelectedMarker")
	}

	query = query.Where("relation_id = ?", relation.ID)

	query = query.Where("marker_id", markerId)

	if err := query.Find(&schedules).Error; err != nil {
		return schedules, err
	}

	return schedules, nil
}
