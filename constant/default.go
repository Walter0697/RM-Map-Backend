package constant

const (
	// pin type
	RegularPin   string = "regular"
	FavouritePin string = "favourite"
	SchedulePin  string = "schedule"
	HurryPin     string = "hurry"
	// update default type
	PinType string = "pin"
)

func GetDefaultPinList() []string {
	return []string{RegularPin, FavouritePin, SchedulePin, HurryPin}
}
