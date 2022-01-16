package scrapper

import (
	"encoding/json"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/service"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type OpenriceRating struct {
	Like    string `json:"like"`
	Average string `json:"average"`
	Dislike string `json:"dislike"`
}

type OpenriceOpening struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

func GetDataFromOpenrice(id string) (*dbmodel.Restaurant, error) {
	baseUrl := "https://s.openrice.com/"
	link := baseUrl + id

	res, err := http.Get(link)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var restaurantModel dbmodel.Restaurant

	doc.Find(".breadcrumb").Each(func(x int, banner *goquery.Selection) {
		banner.Find("li").Each(func(y int, li *goquery.Selection) {
			li.Find("meta").Each(func(z int, s *goquery.Selection) {
				property, _ := s.Attr("itemprop")
				if strings.EqualFold(property, "name") {
					content, _ := s.Attr("content")
					restaurantModel.Name = content
				}
			})
		})
	})

	doc.Find(".header-poi-price.dot-separator").Each(func(i int, s *goquery.Selection) {
		priceTag := s.Find("a")
		restaurantModel.PriceRange = service.ImmediateText(priceTag)
	})

	doc.Find(".introduction-section").Each(func(i int, s *goquery.Selection) {
		introductionTag := s.Find(".content.js-text-wrapper")
		restaurantModel.Introduction = service.ImmediateText(introductionTag)
	})

	doc.Find(".header-poi-categories.dot-separator").Each(func(i int, s *goquery.Selection) {
		categoryTag := s.Find("a")
		restaurantModel.RestaurantType = service.ImmediateText(categoryTag)
	})

	var scoreRating OpenriceRating
	var scoreIndex = 0
	doc.Find(".pois-detail-header.js-pois-detail-header").Each(func(i int, s *goquery.Selection) {
		scoreinner := s.Find(".score-promotion-section")
		scoreinner.Find(".score-div").Each(func(i int, w *goquery.Selection) {
			scoreIndex += 1
			if scoreIndex == 1 {
				scoreRating.Like = service.ImmediateText(w)
			} else if scoreIndex == 2 {
				scoreRating.Average = service.ImmediateText(w)
			} else {
				scoreRating.Dislike = service.ImmediateText(w)
			}
		})
	})
	scoreStr, err := json.Marshal(scoreRating)
	if err == nil {
		restaurantModel.Rating = string(scoreStr)
	}

	doc.Find(".address-info-section").Each(func(i int, s *goquery.Selection) {
		content := s.Find(".content")
		address := content.Find("a")
		restaurantModel.Address = service.ImmediateText(address)
	})

	doc.Find(".telephone-section").Each(func(i int, s *goquery.Selection) {
		content := s.Find(".content")
		restaurantModel.Telephone = service.ImmediateText(content)
	})

	var payment []string
	var condition []string
	doc.Find("#pois-filter-expandable-features").Each(func(x int, s *goquery.Selection) {
		s.Find(".comma-tags").Each(func(y int, pm *goquery.Selection) {
			pm.Find("span").Each(func(z int, span *goquery.Selection) {
				payment = append(payment, service.ImmediateText(span))
			})
		})

		s.Find(".condition-item").Each(func(y int, con *goquery.Selection) {
			item := con.Find(".condition-name")
			condition = append(condition, service.ImmediateText(item))
		})

		content := s.Find(".content")
		restaurantModel.SeatNo = service.ImmediateText(content)
	})
	if len(payment) != 0 {
		restaurantModel.PaymentMethod = strings.Join(payment, "/")
	}
	if len(condition) != 0 {
		restaurantModel.OtherInfo = strings.Join(condition, "/")
	}

	var openingTime []OpenriceOpening
	doc.Find(".opening-hours-list").Each(func(i int, list *goquery.Selection) {
		list.Find(".opening-hours-day").Each(func(i int, opening *goquery.Selection) {
			date := opening.Find(".opening-hours-date")
			time := opening.Find(".opening-hours-time")
			innerTime := time.Find("div")

			var oo OpenriceOpening
			oo.Date = service.ImmediateText(date)
			oo.Time = service.ImmediateText(innerTime)
			openingTime = append(openingTime, oo)
		})
	})
	if len(openingTime) != 0 {
		openingStr, err := json.Marshal(openingTime)
		if err == nil {
			restaurantModel.OpeningHours = string(openingStr)
		}
	}

	doc.Find(".transport-section").Each(func(i int, s *goquery.Selection) {
		transportTag := s.Find("div")
		restaurantModel.Direction = service.ImmediateText(transportTag)
	})

	doc.Find(".restaurant-url-section").Each(func(i int, s *goquery.Selection) {
		urlTag := s.Find("a")
		restaurantModel.Website = service.ImmediateText(urlTag)
	})

	return &restaurantModel, nil
}
