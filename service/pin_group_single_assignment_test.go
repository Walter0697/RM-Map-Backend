package service

import "testing"

func TestConvertGroupIDInputRejectsMultipleGroups(t *testing.T) {
	_, err := convertGroupIDInput([]int{1, 2})
	if err == nil {
		t.Fatalf("expected error when assigning more than one group")
	}
	if err.Error() != ErrPinGroupSingleAssignmentOnly.Error() {
		t.Fatalf("expected %q, got %q", ErrPinGroupSingleAssignmentOnly.Error(), err.Error())
	}
}
