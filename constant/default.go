package constant

const (
	// pin type
	RegularPin   string = "regular"
	SelectedPin  string = "selected"
	FavouritePin string = "favourite"
	HurryPin     string = "hurry"
	// update default type
	PinType string = "pin"
)

func GetDefaultPinList() []string {
	return []string{RegularPin, SelectedPin, FavouritePin, HurryPin}
}
