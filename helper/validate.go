package helper

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/utils"
)

func ValidateScheduleStatus(input model.ScheduleStatusList) error {
	allowed_status := constant.GetStatusList()

	for _, info := range input.Ids {
		if !utils.StringInSlice(info.Status, allowed_status) {
			return &InvalidStatusError{}
		}
	}

	return nil
}
