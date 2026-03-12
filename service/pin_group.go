package service

import (
	"errors"
	"fmt"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"strings"

	"gorm.io/gorm"
)

var ErrPinGroupNameRequired = errors.New("pin group name is required")
var ErrPinGroupDuplicateName = errors.New("pin group name already exists")
var ErrPinGroupNotFound = errors.New("pin group not found")
var ErrPinGroupInvalidAssignment = errors.New("pin group assignment contains unknown group ids")
var ErrPinGroupSingleAssignmentOnly = errors.New("only one pin group can be assigned to a pin")

func normalizePinGroupName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func ListPinGroups() ([]dbmodel.PinGroup, error) {
	items := make([]dbmodel.PinGroup, 0)
	if err := database.Connection.Order("name asc").Find(&items).Error; err != nil {
		return items, err
	}
	return items, nil
}

func CreatePinGroup(name string, isNew bool, actor *dbmodel.User) (*dbmodel.PinGroup, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, ErrPinGroupNameRequired
	}

	group := dbmodel.PinGroup{
		Name:           trimmed,
		IsNew:          isNew,
		NormalizedName: normalizePinGroupName(trimmed),
	}
	if actor != nil {
		group.CreatedBy = actor
		group.UpdatedBy = actor
	}

	if err := group.Create(database.Connection); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, ErrPinGroupDuplicateName
		}
		return nil, err
	}
	return &group, nil
}

func UpdatePinGroup(groupID uint, name string, isNew bool, actor *dbmodel.User) (*dbmodel.PinGroup, error) {
	if groupID == 0 {
		return nil, ErrPinGroupNotFound
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, ErrPinGroupNameRequired
	}

	var existing dbmodel.PinGroup
	existing.ID = groupID
	if err := existing.GetByID(database.Connection); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPinGroupNotFound
		}
		return nil, err
	}

	existing.Name = trimmed
	existing.IsNew = isNew
	existing.NormalizedName = normalizePinGroupName(trimmed)
	if actor != nil {
		existing.UpdatedBy = actor
	}
	if err := existing.Update(database.Connection); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, ErrPinGroupDuplicateName
		}
		return nil, err
	}

	return &existing, nil
}

func DeletePinGroup(groupID uint) error {
	if groupID == 0 {
		return ErrPinGroupNotFound
	}

	var existing dbmodel.PinGroup
	existing.ID = groupID
	if err := existing.GetByID(database.Connection); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPinGroupNotFound
		}
		return err
	}

	// Explicit cleanup keeps behavior deterministic even if DB constraints differ by environment.
	if err := database.Connection.Where("pin_group_id = ?", groupID).Delete(&dbmodel.PinGroupAssignment{}).Error; err != nil {
		return err
	}

	if err := existing.RemoveByID(database.Connection); err != nil {
		return err
	}
	return nil
}

func SetPinGroupAssignments(pinID uint, groupIDs []uint, actor *dbmodel.User) error {
	var pin dbmodel.Pin
	pin.ID = pinID
	if err := pin.GetById(database.Connection); err != nil {
		return err
	}

	cleaned := make([]uint, 0, len(groupIDs))
	seen := make(map[uint]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	if len(cleaned) > 1 {
		return ErrPinGroupSingleAssignmentOnly
	}

	groups := make([]dbmodel.PinGroup, 0, len(cleaned))
	if len(cleaned) > 0 {
		if err := database.Connection.Where("id IN ?", cleaned).Find(&groups).Error; err != nil {
			return err
		}
		if len(groups) != len(cleaned) {
			return ErrPinGroupInvalidAssignment
		}
	}

	tx := database.Connection.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Where("pin_id = ?", pinID).Delete(&dbmodel.PinGroupAssignment{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	actorID := (*uint)(nil)
	if actor != nil {
		actorID = &actor.ID
	}
	for _, groupID := range cleaned {
		entry := dbmodel.PinGroupAssignment{
			PinID:      pinID,
			PinGroupID: groupID,
			CreatedUID: actorID,
		}
		if err := tx.Create(&entry).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}
	return nil
}

func GetPinGroupAssignments(pinID uint) ([]dbmodel.PinGroup, error) {
	var pin dbmodel.Pin
	pin.ID = pinID
	if err := database.Connection.Preload("Groups").First(&pin).Error; err != nil {
		return nil, err
	}

	items := pin.Groups
	if len(items) == 0 {
		return []dbmodel.PinGroup{}, nil
	}
	return items, nil
}

func BuildGroupedPinSections(pins []dbmodel.Pin) []map[string]interface{} {
	grouped := make(map[uint][]dbmodel.Pin)
	groupNames := make(map[uint]string)
	ungrouped := make([]dbmodel.Pin, 0)

	for _, pin := range pins {
		if len(pin.Groups) == 0 {
			ungrouped = append(ungrouped, pin)
			continue
		}
		for _, group := range pin.Groups {
			grouped[group.ID] = append(grouped[group.ID], pin)
			groupNames[group.ID] = group.Name
		}
	}

	sections := make([]map[string]interface{}, 0, len(grouped)+1)
	for groupID, list := range grouped {
		sections = append(sections, map[string]interface{}{
			"group_id":   groupID,
			"group_name": groupNames[groupID],
			"pins":       list,
		})
	}
	if len(ungrouped) > 0 {
		sections = append(sections, map[string]interface{}{
			"group_id":   nil,
			"group_name": "Ungrouped",
			"pins":       ungrouped,
		})
	}
	return sections
}

func ValidatePinGroupIDs(groupIDs []uint) error {
	if len(groupIDs) == 0 {
		return nil
	}
	count := int64(0)
	if err := database.Connection.Model(&dbmodel.PinGroup{}).Where("id IN ?", groupIDs).Count(&count).Error; err != nil {
		return err
	}
	if int(count) != len(groupIDs) {
		return fmt.Errorf("%w: expected %d got %d", ErrPinGroupInvalidAssignment, len(groupIDs), count)
	}
	return nil
}
