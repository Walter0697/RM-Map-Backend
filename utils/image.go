package utils

import (
	"bytes"
	"crypto/tls"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
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

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	rawBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	compareReader := ioutil.NopCloser(bytes.NewBuffer(rawBody))
	origin_size, should_compress := ShouldCompress(compareReader)
	response.Body = ioutil.NopCloser(bytes.NewBuffer(rawBody))

	if should_compress {
		extension := filepath.Ext(url)

		compress_ratio := CompressRatio(int32(origin_size), int32(TargetMemory))
		compressed, err := CompressImage(response.Body, compress_ratio, extension)
		if err != nil {
			return err
		}

		if compressed != nil {
			_, err = io.Copy(out, compressed)
			if err != nil {
				return err
			}
		} else {
			// even if the item cannot be compressed, we should still save it
			response.Body = ioutil.NopCloser(bytes.NewBuffer(rawBody))
			_, err = io.Copy(out, response.Body)
			if err != nil {
				return err
			}
		}

	} else {
		_, err = io.Copy(out, response.Body)
		if err != nil {
			return err
		}
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
