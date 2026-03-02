package constant

const (
	// pin type
	RegularPin   string = "regular"
	SelectedPin  string = "selected"
	FavouritePin string = "favourite"
	HurryPin     string = "hurry"
	PreviewPin   string = "preview"
	// update default type
	PinType string = "pin"
)

func GetDefaultPinList() []string {
	return []string{RegularPin, SelectedPin, FavouritePin, HurryPin, PreviewPin}
}
