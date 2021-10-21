package constant

import "time"

const (
	BasePath          string = "./uploads/"
	MarkerPreviewPath string = "markers"
)

func GetImageName(filetype string, extension string) string {
	now := time.Now()
	return BasePath + filetype + now.Format("2006-01-02T15-04-05-07-00") + "." + extension
}
