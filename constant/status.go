package constant

const (
	Empty     string = ""
	Arrived   string = "arrived"
	Cancelled string = "cancelled"
	Scheduled string = "scheduled"
)

func GetStatusList() []string {
	return []string{Empty, Arrived, Cancelled}
}
