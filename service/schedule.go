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

	marker.Status = constant.Scheduled
	if err := marker.Update(tx); err != nil {
		return nil, err
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

func GetAllSchedule(requested []string, relation dbmodel.UserRelation) ([]dbmodel.Schedule, error) {
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
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	query = query.Where("selected_date > ?", today.Format(utils.StandardTime))

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

		// update marker based on the current status as well
		schedule.SelectedMarker.UpdatedBy = &user
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
		schedule.UpdatedBy = &user

		if err := schedule.Update(tx); err != nil {
			return schedules, err
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// this function is used to get all yesterday schedules
func GetYesterdaySchedules(requested []string, relation dbmodel.UserRelation) ([]dbmodel.Schedule, error) {
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

	// filter only yesterday schedules
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	query = query.Where("selected_date >= ?", start.Format(utils.StandardTime)).
		Where("selected_date < ?", end.Format(utils.StandardTime))

	if err := query.Find(&schedules).Error; err != nil {
		return schedules, err
	}

	return schedules, nil
}
