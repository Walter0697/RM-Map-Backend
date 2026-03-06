package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"

	"github.com/disintegration/imaging"
	"gorm.io/gorm"
)

const (
	defaultStaticPreviewWidth   = 600
	defaultStaticPreviewHeight  = 400
	defaultStaticPreviewZoom    = 14
	defaultStaticPreviewFormat  = "png"
	defaultStaticPreviewStyle   = "main"
	defaultStaticPreviewLayer   = "basic"
	defaultStaticPreviewTimeout = 8 * time.Second
)

var staticPreviewHTTPClient = &http.Client{Timeout: defaultStaticPreviewTimeout}

var (
	ErrInvalidCoordinates = errors.New("invalid coordinates")
	ErrTomTomStaticMap    = errors.New("tomtom static map failure")
	ErrImageComposition   = errors.New("image composition failure")
	ErrMarkerTypeNotFound = errors.New("marker type not found")
)

var resolveUserPreviewPinByUsernameFn = ResolveUserPreviewPinByUsername
var fetchTomTomStaticMapImageFn = fetchTomTomStaticMapImage
var composeStaticPreviewImageFn = composeStaticPreviewImage
var resolveMarkerTypeByNameFn = resolveMarkerTypeByName

type staticPreviewResult struct {
	User     *dbmodel.User
	Pin      *dbmodel.Pin
	Image    []byte
	Format   string
	MimeType string
	Width    int
	Height   int
}

func GenerateStaticMapPreviewByUsername(username string, markerTypeName string, lat float64, lon float64) (*staticPreviewResult, error) {
	if err := validateCoordinates(lat, lon); err != nil {
		return nil, err
	}

	user, pin, err := resolveUserPreviewPinByUsernameFn(username)
	if err != nil {
		return nil, err
	}
	pinImagePath := strings.TrimSpace(pin.ImagePath)
	if pinImagePath == "" {
		return nil, ErrPreviewPinInvalid
	}

	var markerType *dbmodel.MarkerType
	markerTypeName = strings.TrimSpace(markerTypeName)
	if markerTypeName != "" {
		markerType, err = resolveMarkerTypeByNameFn(markerTypeName)
		if err != nil {
			return nil, err
		}
	}

	staticBytes, err := fetchTomTomStaticMapImageFn(lat, lon)
	if err != nil {
		return nil, err
	}

	composed, width, height, err := composeStaticPreviewImageFn(staticBytes, *pin, markerType)
	if err != nil {
		return nil, err
	}

	return &staticPreviewResult{
		User:     user,
		Pin:      pin,
		Image:    composed,
		Format:   defaultStaticPreviewFormat,
		MimeType: "image/png",
		Width:    width,
		Height:   height,
	}, nil
}

func StaticPreviewToBase64(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

func validateCoordinates(lat float64, lon float64) error {
	if lat == 0 && lon == 0 {
		return ErrInvalidCoordinates
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return ErrInvalidCoordinates
	}
	return nil
}

func fetchTomTomStaticMapImage(lat float64, lon float64) ([]byte, error) {
	if strings.TrimSpace(config.Data.APIKEY.TomTomMap) == "" {
		return nil, fmt.Errorf("%w: api key is not configured", ErrTomTomStaticMap)
	}

	endpoint, err := url.Parse("https://api.tomtom.com/map/1/staticimage")
	if err != nil {
		return nil, err
	}

	query := endpoint.Query()
	query.Set("key", config.Data.APIKEY.TomTomMap)
	query.Set("layer", defaultStaticPreviewLayer)
	query.Set("style", defaultStaticPreviewStyle)
	query.Set("format", defaultStaticPreviewFormat)
	query.Set("zoom", fmt.Sprintf("%d", defaultStaticPreviewZoom))
	query.Set("center", fmt.Sprintf("%f,%f", lon, lat))
	query.Set("width", fmt.Sprintf("%d", defaultStaticPreviewWidth))
	query.Set("height", fmt.Sprintf("%d", defaultStaticPreviewHeight))
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	response, err := staticPreviewHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed: %v", ErrTomTomStaticMap, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: status %d", ErrTomTomStaticMap, response.StatusCode)
	}

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrTomTomStaticMap, err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("%w: empty response", ErrTomTomStaticMap)
	}

	return payload, nil
}

func composeStaticPreviewImage(staticMapImage []byte, pin dbmodel.Pin, markerType *dbmodel.MarkerType) ([]byte, int, int, error) {
	overlayImagePath := strings.TrimSpace(pin.ImagePath)
	if overlayImagePath == "" {
		return nil, 0, 0, fmt.Errorf("%w: overlay image path is required", ErrImageComposition)
	}

	baseImage, err := imaging.Decode(bytes.NewReader(staticMapImage))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: decode static map: %v", ErrImageComposition, err)
	}

	resolvedPath := filepath.Join(constant.BasePath, strings.TrimPrefix(strings.TrimSpace(overlayImagePath), "/"))
	pinImage, err := imaging.Open(resolvedPath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: open overlay image: %v", ErrImageComposition, err)
	}

	if markerType != nil {
		typeIconPath := strings.TrimSpace(markerType.IconPath)
		if typeIconPath == "" {
			return nil, 0, 0, fmt.Errorf("%w: marker type icon path is required", ErrImageComposition)
		}

		typeIconResolvedPath := filepath.Join(constant.BasePath, strings.TrimPrefix(typeIconPath, "/"))
		typeImage, typeErr := imaging.Open(typeIconResolvedPath)
		if typeErr != nil {
			return nil, 0, 0, fmt.Errorf("%w: open marker type image: %v", ErrImageComposition, typeErr)
		}

		iconWidth := pin.BottomRightX - pin.TopLeftX
		iconHeight := pin.BottomRightY - pin.TopLeftY
		if iconWidth <= 0 || iconHeight <= 0 {
			return nil, 0, 0, fmt.Errorf("%w: pin icon bounds are invalid", ErrImageComposition)
		}

		smallTypeIcon := imaging.Resize(typeImage, iconWidth, iconHeight, imaging.Lanczos)
		pinImage = imaging.Overlay(pinImage, smallTypeIcon, imagePoint(pin.TopLeftX, pin.TopLeftY), 1.0)
	}

	pinOverlay := imaging.Fit(pinImage, 96, 96, imaging.Lanczos)
	baseBounds := baseImage.Bounds()
	overlayBounds := pinOverlay.Bounds()
	offsetX := (baseBounds.Dx() - overlayBounds.Dx()) / 2
	offsetY := (baseBounds.Dy() - overlayBounds.Dy()) / 2
	finalImage := imaging.Overlay(baseImage, pinOverlay, imagePoint(offsetX, offsetY), 1.0)

	buffer := &bytes.Buffer{}
	if err := imaging.Encode(buffer, finalImage, imaging.PNG); err != nil {
		return nil, 0, 0, fmt.Errorf("%w: encode composed image: %v", ErrImageComposition, err)
	}

	return buffer.Bytes(), baseBounds.Dx(), baseBounds.Dy(), nil
}

func imagePoint(x int, y int) image.Point {
	return image.Pt(x, y)
}

func resolveMarkerTypeByName(markerTypeName string) (*dbmodel.MarkerType, error) {
	name := strings.TrimSpace(markerTypeName)
	if name == "" {
		return nil, ErrMarkerTypeNotFound
	}

	markerType := dbmodel.MarkerType{}
	if err := database.Connection.Where("LOWER(value) = LOWER(?) OR LOWER(label) = LOWER(?)", name, name).First(&markerType).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMarkerTypeNotFound
		}
		return nil, err
	}

	return &markerType, nil
}
