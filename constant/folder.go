package constant

import (
	"fmt"
	"time"
)

const (
	BasePath          string = "./uploads"
	MarkerPreviewPath string = "/markers/"
	TypeIconPath      string = "/types/"
	PinImagePath      string = "/pins/"
	PreviewImagePath  string = "/previews/"
	TypePinImagePath  string = "/typepins/"
)

func GetImageName(filetype string, extension string) string {
	now := time.Now()
	return filetype + now.Format("2006-01-02T15-04-05-07-00") + "." + extension
}

func GetPinDisplayName(extension string) string {
	now := time.Now()
	return PinImagePath + "P" + now.Format("2006-01-02T15-04-05-07-00") + "." + extension
}

func TypePinImageName(typeid, pinid int, extension string) string {
	filename := fmt.Sprintf("%s%d-%d.%s", TypePinImagePath, pinid, typeid, extension)
	return filename
}
