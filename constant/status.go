package constant

const (
	Empty     string = ""
	Arrived   string = "arrived"
	Cancelled string = "cancelled"
)

func GetStatusList() []string {
	return []string{Empty, Arrived, Cancelled}
}
