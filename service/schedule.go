package service

import (
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"
	"time"

	"gorm.io/gorm"
)

func CreateSchedule(input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.Schedule, error) {
	var schedule dbmodel.Schedule

	schedule.Label = input.Label
	schedule.Description = input.Description

	schedule.SelectedMarker = &marker

	selectedTime, err := time.Parse(time.RFC3339, input.SelectedTime)
	if err != nil {
		return nil, err
	}
	schedule.SelectedDate = selectedTime

	schedule.Relation = relation
	schedule.Status = ""
	schedule.CreatedBy = &user
	schedule.UpdatedBy = &user

	if err := schedule.Create(); err != nil {
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
		if err := schedule.GetByIdWithTransaction(tx); err != nil {
			return schedules, helper.GetDatabaseError(err)
		}

		if schedule.RelationId != relation.ID {
			return schedules, &helper.InvalidRelationUpdateError{}
		}

		schedule.Status = updateStatus.Status
		if updateStatus.Status == "arrived" {
			schedule.SelectedMarker.Status = updateStatus.Status
			schedule.SelectedMarker.UpdatedBy = &user

			if err := schedule.SelectedMarker.UpdateWithTransaction(tx); err != nil {
				return schedules, err
			}
		}
		schedule.UpdatedBy = &user

		if err := schedule.UpdateWithTransaction(tx); err != nil {
			return schedules, err
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}
