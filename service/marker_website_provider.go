package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type MarkerWebsiteProvider interface {
	ID() string
	ValidateExternalID(externalID string) error
	FetchRestaurant(externalID string) (*dbmodel.Restaurant, error)
}

var (
	openriceExternalIDPattern = regexp.MustCompile(`^[A-Za-z0-9/_-]{3,200}$`)
	yelpExternalIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{3,200}$`)
	tabelogExternalIDPattern  = regexp.MustCompile(`^[A-Za-z0-9/_-]{3,220}$`)
)

var markerWebsiteProviders = map[string]MarkerWebsiteProvider{
	constant.Openrice: openriceWebsiteProvider{},
	constant.Yelp:     yelpWebsiteProvider{},
	constant.Tabelog:  tabelogWebsiteProvider{},
}

func NormalizeMarkerWebsiteProviderID(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func GetMarkerWebsiteProvider(provider string) (MarkerWebsiteProvider, bool) {
	item, ok := markerWebsiteProviders[NormalizeMarkerWebsiteProviderID(provider)]
	return item, ok
}

func ValidateMarkerWebsiteIntegration(provider string, externalID string) error {
	providerID := NormalizeMarkerWebsiteProviderID(provider)
	externalID = strings.TrimSpace(externalID)
	if providerID == "" && externalID == "" {
		return nil
	}
	if providerID == "" || externalID == "" {
		return fmt.Errorf("website_provider and website_provider_id are required together")
	}
	item, ok := GetMarkerWebsiteProvider(providerID)
	if !ok {
		return fmt.Errorf("unsupported website provider: %s", providerID)
	}
	return item.ValidateExternalID(externalID)
}

func GetOrCreateRestaurantByProvider(provider string, externalID string) (*dbmodel.Restaurant, error) {
	providerID := NormalizeMarkerWebsiteProviderID(provider)
	externalID = strings.TrimSpace(externalID)

	if err := ValidateMarkerWebsiteIntegration(providerID, externalID); err != nil {
		return nil, err
	}
	if providerID == "" || externalID == "" {
		return nil, nil
	}

	existing := dbmodel.Restaurant{
		Source:   providerID,
		SourceId: externalID,
	}
	if err := existing.GetBySourceIdAndSource(database.Connection); err == nil {
		return &existing, nil
	}

	item, ok := GetMarkerWebsiteProvider(providerID)
	if !ok {
		return nil, fmt.Errorf("unsupported website provider: %s", providerID)
	}
	restaurant, err := item.FetchRestaurant(externalID)
	if err != nil {
		return nil, err
	}
	restaurant.Source = providerID
	restaurant.SourceId = externalID
	if err := restaurant.Create(database.Connection); err != nil {
		return nil, err
	}
	return restaurant, nil
}

type openriceWebsiteProvider struct{}

func (openriceWebsiteProvider) ID() string {
	return constant.Openrice
}

func (openriceWebsiteProvider) ValidateExternalID(externalID string) error {
	value := strings.TrimSpace(externalID)
	if !openriceExternalIDPattern.MatchString(value) || strings.Contains(strings.ToLower(value), "http") {
		return fmt.Errorf("invalid OpenRice source id format")
	}
	return nil
}

func (openriceWebsiteProvider) FetchRestaurant(externalID string) (*dbmodel.Restaurant, error) {
	restaurant := &dbmodel.Restaurant{
		Source:   constant.Openrice,
		SourceId: strings.TrimSpace(externalID),
	}
	body, err := GetRequestWithExternalAPIAudit(constant.Openrice, "restaurant_details", "https://s.openrice.com/"+restaurant.SourceId, nil)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	doc.Find(".breadcrumb").Each(func(x int, banner *goquery.Selection) {
		banner.Find("li").Each(func(y int, li *goquery.Selection) {
			li.Find("meta").Each(func(z int, s *goquery.Selection) {
				property, _ := s.Attr("itemprop")
				if strings.EqualFold(property, "name") {
					content, _ := s.Attr("content")
					restaurant.Name = content
				}
			})
		})
	})

	doc.Find(".header-poi-price.dot-separator").Each(func(i int, s *goquery.Selection) {
		restaurant.PriceRange = immediateText(s.Find("a"))
	})

	doc.Find(".introduction-section").Each(func(i int, s *goquery.Selection) {
		restaurant.Introduction = immediateText(s.Find(".content.js-text-wrapper"))
	})

	doc.Find(".header-poi-categories.dot-separator").Each(func(i int, s *goquery.Selection) {
		restaurant.RestaurantType = immediateText(s.Find("a"))
	})

	var scoreRating struct {
		Like    string `json:"like"`
		Average string `json:"average"`
		Dislike string `json:"dislike"`
	}
	scoreIndex := 0
	doc.Find(".pois-detail-header.js-pois-detail-header").Each(func(i int, s *goquery.Selection) {
		scoreInner := s.Find(".score-promotion-section")
		scoreInner.Find(".score-div").Each(func(i int, w *goquery.Selection) {
			scoreIndex++
			if scoreIndex == 1 {
				scoreRating.Like = immediateText(w)
			} else if scoreIndex == 2 {
				scoreRating.Average = immediateText(w)
			} else {
				scoreRating.Dislike = immediateText(w)
			}
		})
	})
	if scoreStr, marshalErr := json.Marshal(scoreRating); marshalErr == nil {
		restaurant.Rating = string(scoreStr)
	}

	doc.Find(".address-info-section").Each(func(i int, s *goquery.Selection) {
		restaurant.Address = immediateText(s.Find(".content").Find("a"))
	})

	var phone []string
	doc.Find(".telephone-section").Each(func(i int, s *goquery.Selection) {
		s.Find(".content").Each(func(j int, content *goquery.Selection) {
			phone = append(phone, immediateText(content))
		})
	})
	if len(phone) != 0 {
		restaurant.Telephone = strings.Join(phone, "/")
	}

	var payment []string
	var condition []string
	doc.Find("#pois-filter-expandable-features").Each(func(x int, s *goquery.Selection) {
		s.Find(".comma-tags").Each(func(y int, pm *goquery.Selection) {
			pm.Find("span").Each(func(z int, span *goquery.Selection) {
				payment = append(payment, immediateText(span))
			})
		})
		s.Find(".condition-item").Each(func(y int, con *goquery.Selection) {
			condition = append(condition, immediateText(con.Find(".condition-name")))
		})
		restaurant.SeatNo = immediateText(s.Find(".content"))
	})
	if len(payment) != 0 {
		restaurant.PaymentMethod = strings.Join(payment, "/")
	}
	if len(condition) != 0 {
		restaurant.OtherInfo = strings.Join(condition, "/")
	}

	doc.Find(".transport-section").Each(func(i int, s *goquery.Selection) {
		restaurant.Direction = immediateText(s.Find("div"))
	})

	doc.Find(".restaurant-url-section").Each(func(i int, s *goquery.Selection) {
		restaurant.Website = immediateText(s.Find("a"))
	})

	return restaurant, nil
}

type yelpWebsiteProvider struct{}

func (yelpWebsiteProvider) ID() string {
	return constant.Yelp
}

func (yelpWebsiteProvider) ValidateExternalID(externalID string) error {
	value := strings.TrimSpace(externalID)
	if !yelpExternalIDPattern.MatchString(value) || strings.Contains(value, "/") {
		return fmt.Errorf("invalid Yelp business id format")
	}
	return nil
}

func (yelpWebsiteProvider) FetchRestaurant(externalID string) (*dbmodel.Restaurant, error) {
	id := strings.TrimSpace(externalID)
	rawURL := "https://www.yelp.com/biz/" + url.PathEscape(id)
	body, err := GetRequestWithExternalAPIAudit(ExternalAPIProviderYelp, "business_details", rawURL, nil)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	restaurant := &dbmodel.Restaurant{
		Source:   constant.Yelp,
		SourceId: id,
		Website:  rawURL,
	}

	title := strings.TrimSpace(doc.Find("meta[property='og:title']").AttrOr("content", ""))
	title = strings.TrimSuffix(title, " - Yelp")
	if title != "" {
		restaurant.Name = title
	}
	if restaurant.Name == "" {
		restaurant.Name = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	rating := strings.TrimSpace(doc.Find("meta[itemprop='ratingValue']").AttrOr("content", ""))
	if rating == "" {
		rating = strings.TrimSpace(doc.Find("meta[property='og:rating']").AttrOr("content", ""))
	}
	if rating != "" {
		restaurant.Rating = rating
	}

	address := strings.TrimSpace(doc.Find("meta[property='business:contact_data:street_address']").AttrOr("content", ""))
	if address == "" {
		address = strings.TrimSpace(doc.Find("address").First().Text())
	}
	if address != "" {
		restaurant.Address = address
	}

	telephone := strings.TrimSpace(doc.Find("meta[property='business:contact_data:phone_number']").AttrOr("content", ""))
	if telephone != "" {
		restaurant.Telephone = telephone
	}

	if restaurant.Website == "" {
		restaurant.Website = rawURL
	}

	return restaurant, nil
}

type tabelogWebsiteProvider struct{}

func (tabelogWebsiteProvider) ID() string {
	return constant.Tabelog
}

func (tabelogWebsiteProvider) ValidateExternalID(externalID string) error {
	value := strings.TrimSpace(externalID)
	if !tabelogExternalIDPattern.MatchString(value) || strings.Contains(strings.ToLower(value), "http") {
		return fmt.Errorf("invalid Tabelog restaurant id format")
	}
	return nil
}

func (tabelogWebsiteProvider) FetchRestaurant(externalID string) (*dbmodel.Restaurant, error) {
	id := strings.TrimSpace(externalID)
	rawURL := "https://tabelog.com/" + strings.TrimPrefix(id, "/")
	body, err := GetRequestWithExternalAPIAudit(ExternalAPIProviderTabelog, "restaurant_details", rawURL, nil)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	restaurant := &dbmodel.Restaurant{
		Source:   constant.Tabelog,
		SourceId: id,
		Website:  rawURL,
	}

	title := strings.TrimSpace(doc.Find("meta[property='og:title']").AttrOr("content", ""))
	title = strings.TrimSuffix(title, " - 食べログ")
	title = strings.TrimSuffix(title, " - Tabelog")
	if title != "" {
		restaurant.Name = title
	}
	if restaurant.Name == "" {
		restaurant.Name = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	rating := strings.TrimSpace(doc.Find("meta[property='og:rating']").AttrOr("content", ""))
	if rating == "" {
		rating = strings.TrimSpace(doc.Find("meta[itemprop='ratingValue']").AttrOr("content", ""))
	}
	if rating != "" {
		restaurant.Rating = rating
	}

	address := strings.TrimSpace(doc.Find("meta[property='restaurant:contact_info:street_address']").AttrOr("content", ""))
	if address == "" {
		address = strings.TrimSpace(doc.Find("address").First().Text())
	}
	if address != "" {
		restaurant.Address = address
	}

	telephone := strings.TrimSpace(doc.Find("meta[property='restaurant:contact_info:phone_number']").AttrOr("content", ""))
	if telephone != "" {
		restaurant.Telephone = telephone
	}

	if restaurant.Website == "" {
		restaurant.Website = rawURL
	}

	return restaurant, nil
}

func immediateText(s *goquery.Selection) string {
	var buf bytes.Buffer
	for _, node := range s.Nodes {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				buf.WriteString(child.Data)
			}
		}
	}
	return strings.TrimSpace(buf.String())
}
