package service

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// first the image, then the title if any
func GetMetaDataFromWebLink(link string) (string, string, error) {
	// if it is not a valid link, then reject it in the beginning
	u, err := url.Parse(link)
	if err != nil {
		return "", "", err
	}

	// fetch the information
	res, err := http.Get(link)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()

	// read it from goquery
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", "", err
	}

	// try to find the meta image
	metaimage := ""
	title := ""
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		property, _ := s.Attr("property")
		if strings.EqualFold(property, "og:title") {
			content, _ := s.Attr("content")

			title = content
		}

		if strings.EqualFold(property, "og:image") {
			content, _ := s.Attr("content")

			metaimage = content
		}
	})

	// if found, return the meta image
	if metaimage != "" {
		return metaimage, title, nil
	}

	domain := u.Hostname()

	image := ""
	// find an image from the image tag
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		name, _ := s.Attr("src")
		image = domain + name
		return
	})

	return image, title, nil
}

func ImmediateText(s *goquery.Selection) string {
	var buf bytes.Buffer

	for _, node := range s.Nodes {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				buf.WriteString(child.Data)
			}
		}
	}

	final := buf.String()
	return strings.TrimSpace(final)
}
