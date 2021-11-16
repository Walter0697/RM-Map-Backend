package service

import (
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/utils"
	"time"
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
