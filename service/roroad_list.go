package service

import (
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/utils"
)

func CreateRoRoadList(input model.NewRoroadList, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.RoRoadList, error) {
	var roroadlist dbmodel.RoRoadList

	roroadlist.Name = input.Name
	roroadlist.TargetUser = input.TargetUser
	roroadlist.Type = input.ListType
	roroadlist.Checked = false
	roroadlist.Hidden = false

	roroadlist.Relation = relation
	roroadlist.CreatedBy = &user
	roroadlist.UpdatedBy = &user

	if err := roroadlist.Create(database.Connection); err != nil {
		return nil, err
	}

	return &roroadlist, nil
}

func EditRoRoadList(input model.UpdateRoroadList, user dbmodel.User) (*dbmodel.RoRoadList, error) {
	var roroadlist dbmodel.RoRoadList

	roroadlist.ID = uint(input.ID)
	if err := roroadlist.GetById(database.Connection); err != nil {
		return nil, err
	}

	if input.Name != nil {
		roroadlist.Name = *input.Name
	}

	if input.TargetUser != nil {
		roroadlist.TargetUser = *input.TargetUser
	}

	if input.ListType != nil {
		roroadlist.Type = *input.ListType
	}

	if input.Checked != nil {
		roroadlist.Checked = *input.Checked
	}

	if input.Hidden != nil {
		roroadlist.Hidden = *input.Hidden
	}

	roroadlist.UpdatedBy = &user

	if err := roroadlist.Update(database.Connection); err != nil {
		return nil, err
	}

	return &roroadlist, nil
}

func UpdateMultipleRoroadList(input model.ManageRoroadList, user dbmodel.User) ([]dbmodel.RoRoadList, error) {
	var roroadlists []dbmodel.RoRoadList

	query := database.Connection

	queryStr := "UPDATE ro_road_lists SET hidden = ? WHERE id in ?"

	if err := query.Raw(queryStr, input.Hidden, input.Ids).Scan(&roroadlists).Error; err != nil {
		return roroadlists, err
	}

	return roroadlists, nil
}

func GetAllActiveRoroadList(requested []string, relation dbmodel.UserRelation) ([]dbmodel.RoRoadList, error) {
	var roroadlists []dbmodel.RoRoadList
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}

	query = query.Where("relation_id = ?", relation.ID)
	query = query.Where("hidden = ?", false)

	if err := query.Find(&roroadlists).Error; err != nil {
		return roroadlists, err
	}

	return roroadlists, nil
}

func FindRoroadListByName(requested []string, name string, relation dbmodel.UserRelation) ([]dbmodel.RoRoadList, error) {
	var roroadlists []dbmodel.RoRoadList
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}

	query = query.Where("relation_id = ?", relation.ID)
	query = query.Where("name = %?%", name)

	if err := query.Find(&roroadlists).Error; err != nil {
		return roroadlists, err
	}

	return roroadlists, nil
}
