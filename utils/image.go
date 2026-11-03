package utils

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	MegaByte     int = 1000000
	TargetMemory int = 30000
)

func ShouldCompress(reader io.Reader) (int, bool) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)

	if buf.Len() > 80000 {
		return buf.Len(), true
	}

	return buf.Len(), false
}

func CompressRatio(original_size int32, target_size int32) float64 {
	target := float64(target_size)
	original := float64(original_size)
	ratio := target / original
	return (math.Floor(ratio*100) / 100)
}

func CompressImage(reader io.ReadCloser, ratio float64, ext string) (*bytes.Buffer, error) {
	switch ext {
	case ".png":
		return CompressPng(reader, ratio)
	case ".jpeg":
	case ".jpg":
		return CompressJpeg(reader, ratio)
	case ".gif":
		return CompressGif(reader, ratio)
	}
	return nil, nil
}

func CompressPng(reader io.ReadCloser, ratio float64) (*bytes.Buffer, error) {
	image, decodeerr := png.Decode(reader)
	if decodeerr != nil {
		return nil, decodeerr
	}

	buff1 := &bytes.Buffer{}
	pngerr := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(buff1, image)

	if pngerr != nil {
		return nil, pngerr
	}

	return buff1, nil
}

func CompressJpeg(reader io.ReadCloser, ratio float64) (*bytes.Buffer, error) {
	image, decodeerr := jpeg.Decode(reader)
	if decodeerr != nil {
		return nil, decodeerr
	}

	buff1 := new(bytes.Buffer)
	quality := int(ratio * 100)
	jpegerr := jpeg.Encode(buff1, image, &jpeg.Options{Quality: quality})
	if jpegerr != nil {
		return nil, jpegerr
	}

	return buff1, nil
}

func CompressGif(reader io.ReadCloser, ratio float64) (*bytes.Buffer, error) {
	image, decodeerr := gif.Decode(reader)
	if decodeerr != nil {
		return nil, decodeerr
	}

	buff1 := new(bytes.Buffer)
	numColor := int(255 * ratio)
	jpegerr := gif.Encode(buff1, image, &gif.Options{NumColors: numColor})
	if jpegerr != nil {
		return nil, jpegerr
	}

	return buff1, nil
}

func SaveImageFromURL(path string, url string) error {
	// WARNING: possibly a dangerous solution but this function will only be used by authorized user
	// I can remove this feature or find another way to work with this
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	response, e := client.Get(url)
	if e != nil {
		return e
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("image request returned HTTP %d", response.StatusCode)
	}

	rawBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	decoded, _, err := image.Decode(bytes.NewReader(rawBody))
	if err != nil {
		return fmt.Errorf("decode downloaded image: %w", err)
	}

	var normalized bytes.Buffer
	switch extension := strings.ToLower(filepath.Ext(path)); extension {
	case ".png":
		err = png.Encode(&normalized, decoded)
	case ".jpg", ".jpeg":
		quality := 85
		if len(rawBody) > 80000 {
			quality = int(CompressRatio(int32(len(rawBody)), int32(TargetMemory)) * 100)
			if quality < 10 {
				quality = 10
			}
			if quality > 95 {
				quality = 95
			}
		}
		err = jpeg.Encode(&normalized, decoded, &jpeg.Options{Quality: quality})
	case ".gif":
		err = gif.Encode(&normalized, decoded, nil)
	default:
		return fmt.Errorf("unsupported image extension %q", extension)
	}
	if err != nil {
		return fmt.Errorf("encode normalized image: %w", err)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, &normalized); err != nil {
		return err
	}

	return nil
}

func UploadImage(path string, file io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		return err
	}
	return nil
}
