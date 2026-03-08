package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
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
		log.Printf(
			"website_provider cache_hit provider=%s source_id=%s url=%s",
			providerID,
			externalID,
			providerDebugURL(providerID, externalID, existing.Website),
		)
		return &existing, nil
	}

	item, ok := GetMarkerWebsiteProvider(providerID)
	if !ok {
		return nil, fmt.Errorf("unsupported website provider: %s", providerID)
	}
	restaurant, err := item.FetchRestaurant(externalID)
	if err != nil {
		log.Printf("website_provider fetch_failed provider=%s source_id=%s err=%v", providerID, externalID, err)
		return nil, err
	}
	restaurant.Source = providerID
	restaurant.SourceId = externalID
	if err := restaurant.Create(database.Connection); err != nil {
		return nil, err
	}
	log.Printf("website_provider created provider=%s source_id=%s restaurant_id=%d", providerID, externalID, restaurant.ID)
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
	rawURL := "https://s.openrice.com/" + restaurant.SourceId
	log.Printf("website_provider fetch provider=%s source_id=%s url=%s", constant.Openrice, restaurant.SourceId, rawURL)
	body, err := GetRequestWithExternalAPIAudit(constant.Openrice, "restaurant_details", rawURL, nil)
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
	apiKey := resolveYelpAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("yelp api key is required; set [apikey].yelp in config.toml or YELP_API_KEY")
	}

	restaurant, err := fetchYelpRestaurantViaAPI(id, apiKey)
	if err != nil {
		return nil, fmt.Errorf("yelp api fetch failed: %w", err)
	}
	return restaurant, nil
}

func resolveYelpAPIKey() string {
	if key := strings.TrimSpace(config.Data.APIKEY.Yelp); key != "" {
		return key
	}
	return strings.TrimSpace(os.Getenv("YELP_API_KEY"))
}

func fetchYelpRestaurantViaAPI(sourceID string, apiKey string) (*dbmodel.Restaurant, error) {
	id := strings.TrimSpace(sourceID)
	rawURL := "https://api.yelp.com/v3/businesses/" + url.PathEscape(id)
	log.Printf("website_provider fetch provider=%s source_id=%s url=%s mode=fusion_api", constant.Yelp, id, rawURL)

	body, err := GetRequestWithExternalAPIAuditAndHeaders(ExternalAPIProviderYelp, "business_details_api", rawURL, map[string]string{
		"Authorization":   "Bearer " + strings.TrimSpace(apiKey),
		"Accept":          "application/json",
		"Accept-Language": "en-US,en;q=0.9",
	}, nil)
	if err != nil {
		return nil, err
	}

	type yelpAPILocation struct {
		Address1       string   `json:"address1"`
		Address2       string   `json:"address2"`
		Address3       string   `json:"address3"`
		City           string   `json:"city"`
		State          string   `json:"state"`
		ZipCode        string   `json:"zip_code"`
		Country        string   `json:"country"`
		CrossStreets   string   `json:"cross_streets"`
		DisplayAddress []string `json:"display_address"`
	}
	type yelpAPICategory struct {
		Alias string `json:"alias"`
		Title string `json:"title"`
	}
	type yelpAPIHoursOpen struct {
		IsOvernight bool   `json:"is_overnight"`
		Start       string `json:"start"`
		End         string `json:"end"`
		Day         int    `json:"day"`
	}
	type yelpAPIHours struct {
		Open      []yelpAPIHoursOpen `json:"open"`
		HoursType string             `json:"hours_type"`
		IsOpenNow bool               `json:"is_open_now"`
	}
	type yelpAPICoordinates struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	type yelpAPIAttributes struct {
		BusinessAcceptsCreditCards interface{} `json:"BusinessAcceptsCreditCards"`
		RestaurantsTakeOut         interface{} `json:"RestaurantsTakeOut"`
		RestaurantsDelivery        interface{} `json:"RestaurantsDelivery"`
		RestaurantsReservations    interface{} `json:"RestaurantsReservations"`
		WheelchairAccessible       interface{} `json:"WheelchairAccessible"`
		OutdoorSeating             interface{} `json:"OutdoorSeating"`
	}
	type yelpAPIResponse struct {
		ID           string             `json:"id"`
		Name         string             `json:"name"`
		ImageURL     string             `json:"image_url"`
		Photos       []string           `json:"photos"`
		URL          string             `json:"url"`
		Phone        string             `json:"phone"`
		DisplayPhone string             `json:"display_phone"`
		Rating       float64            `json:"rating"`
		ReviewCount  int                `json:"review_count"`
		Price        string             `json:"price"`
		Categories   []yelpAPICategory  `json:"categories"`
		Location     yelpAPILocation    `json:"location"`
		Coordinates  yelpAPICoordinates `json:"coordinates"`
		Hours        []yelpAPIHours     `json:"hours"`
		Transactions []string           `json:"transactions"`
		Attributes   yelpAPIAttributes  `json:"attributes"`
	}

	var payload yelpAPIResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	restaurant := &dbmodel.Restaurant{
		Source:   constant.Yelp,
		SourceId: id,
		Name:     strings.TrimSpace(payload.Name),
		PhotoURL: strings.TrimSpace(payload.ImageURL),
		Website:  strings.TrimSpace(payload.URL),
	}
	if restaurant.PhotoURL == "" && len(payload.Photos) > 0 {
		restaurant.PhotoURL = strings.TrimSpace(payload.Photos[0])
	}

	if len(payload.Location.DisplayAddress) > 0 {
		restaurant.Address = strings.TrimSpace(strings.Join(payload.Location.DisplayAddress, ", "))
	} else {
		addressParts := []string{
			strings.TrimSpace(payload.Location.Address1),
			strings.TrimSpace(payload.Location.Address2),
			strings.TrimSpace(payload.Location.Address3),
			strings.TrimSpace(payload.Location.City),
			strings.TrimSpace(payload.Location.State),
			strings.TrimSpace(payload.Location.ZipCode),
			strings.TrimSpace(payload.Location.Country),
		}
		nonEmpty := make([]string, 0, len(addressParts))
		for _, part := range addressParts {
			if part != "" {
				nonEmpty = append(nonEmpty, part)
			}
		}
		restaurant.Address = strings.Join(nonEmpty, ", ")
	}

	if strings.TrimSpace(payload.DisplayPhone) != "" {
		restaurant.Telephone = strings.TrimSpace(payload.DisplayPhone)
	} else {
		restaurant.Telephone = strings.TrimSpace(payload.Phone)
	}

	if payload.Rating > 0 {
		if payload.ReviewCount > 0 {
			restaurant.Rating = fmt.Sprintf("%.2f (%d reviews)", payload.Rating, payload.ReviewCount)
		} else {
			restaurant.Rating = fmt.Sprintf("%.2f", payload.Rating)
		}
	}

	if strings.TrimSpace(payload.Price) != "" {
		restaurant.PriceRange = strings.TrimSpace(payload.Price)
	}
	categoryNames := make([]string, 0, len(payload.Categories))
	for _, item := range payload.Categories {
		name := strings.TrimSpace(item.Title)
		if name != "" {
			categoryNames = append(categoryNames, name)
		}
	}
	if len(categoryNames) > 0 {
		restaurant.RestaurantType = strings.Join(categoryNames, ", ")
	}

	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	openingRows := make([]string, 0)
	for _, hour := range payload.Hours {
		for _, open := range hour.Open {
			dayLabel := "Day" + strconv.Itoa(open.Day)
			if open.Day >= 0 && open.Day < len(dayNames) {
				dayLabel = dayNames[open.Day]
			}
			start := strings.TrimSpace(open.Start)
			end := strings.TrimSpace(open.End)
			if len(start) == 4 {
				start = start[:2] + ":" + start[2:]
			}
			if len(end) == 4 {
				end = end[:2] + ":" + end[2:]
			}
			row := fmt.Sprintf("%s %s-%s", dayLabel, start, end)
			if open.IsOvernight {
				row += " (overnight)"
			}
			openingRows = append(openingRows, row)
		}
	}
	if len(openingRows) > 0 {
		sort.Strings(openingRows)
		restaurant.OpeningHours = strings.Join(openingRows, "; ")
	}

	directionParts := make([]string, 0, 2)
	if cross := normalizeText(payload.Location.CrossStreets); cross != "" {
		directionParts = append(directionParts, "Cross streets: "+cross)
	}
	if payload.Coordinates.Latitude != 0 || payload.Coordinates.Longitude != 0 {
		directionParts = append(directionParts, fmt.Sprintf("Coordinates: %.6f, %.6f", payload.Coordinates.Latitude, payload.Coordinates.Longitude))
	}
	restaurant.Direction = strings.Join(directionParts, " | ")

	paymentBits := make([]string, 0, 2)
	if yelpAttributeBool(payload.Attributes.BusinessAcceptsCreditCards) {
		paymentBits = append(paymentBits, "Accepts credit cards")
	}
	if yelpAttributeBool(payload.Attributes.RestaurantsReservations) {
		paymentBits = append(paymentBits, "Accepts reservations")
	}
	restaurant.PaymentMethod = strings.Join(paymentBits, "; ")

	otherBits := make([]string, 0)
	if len(payload.Transactions) > 0 {
		normalized := make([]string, 0, len(payload.Transactions))
		for _, item := range payload.Transactions {
			name := normalizeText(item)
			if name != "" {
				normalized = append(normalized, name)
			}
		}
		if len(normalized) > 0 {
			otherBits = append(otherBits, "Transactions: "+strings.Join(normalized, ", "))
		}
	}
	if yelpAttributeBool(payload.Attributes.RestaurantsTakeOut) {
		otherBits = append(otherBits, "Takeout available")
	}
	if yelpAttributeBool(payload.Attributes.RestaurantsDelivery) {
		otherBits = append(otherBits, "Delivery available")
	}
	if yelpAttributeBool(payload.Attributes.WheelchairAccessible) {
		otherBits = append(otherBits, "Wheelchair accessible")
	}
	if yelpAttributeBool(payload.Attributes.OutdoorSeating) {
		otherBits = append(otherBits, "Outdoor seating")
	}
	restaurant.OtherInfo = strings.Join(otherBits, " | ")

	introParts := make([]string, 0, 3)
	if payload.ReviewCount > 0 {
		introParts = append(introParts, fmt.Sprintf("%d Yelp reviews", payload.ReviewCount))
	}
	if len(categoryNames) > 0 {
		introParts = append(introParts, "Cuisine: "+strings.Join(categoryNames, ", "))
	}
	if len(payload.Transactions) > 0 {
		introParts = append(introParts, "Services: "+strings.Join(payload.Transactions, ", "))
	}
	restaurant.Introduction = strings.Join(introParts, ". ")

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
	log.Printf("website_provider fetch provider=%s source_id=%s url=%s", constant.Tabelog, id, rawURL)
	body, err := GetRequestWithExternalAPIAuditAndHeaders(ExternalAPIProviderTabelog, "restaurant_details", rawURL, map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9,ja;q=0.8",
		"Referer":         "https://tabelog.com/en/",
	}, nil)
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

	applyTabelogJSONLD(restaurant, doc)

	if strings.TrimSpace(restaurant.Name) == "" {
		title := strings.TrimSpace(doc.Find("meta[property='og:title']").AttrOr("content", ""))
		title = strings.TrimSuffix(title, " - 食べログ")
		title = strings.TrimSuffix(title, " - Tabelog")
		restaurant.Name = normalizeText(title)
	}
	if strings.TrimSpace(restaurant.Name) == "" {
		restaurant.Name = normalizeText(doc.Find("h1").First().Text())
	}

	if strings.TrimSpace(restaurant.Rating) == "" {
		rating := strings.TrimSpace(doc.Find("meta[property='og:rating']").AttrOr("content", ""))
		if rating == "" {
			rating = strings.TrimSpace(doc.Find("meta[itemprop='ratingValue']").AttrOr("content", ""))
		}
		restaurant.Rating = normalizeText(rating)
	}

	if strings.TrimSpace(restaurant.Address) == "" {
		restaurant.Address = normalizeText(doc.Find("address").First().Text())
	}
	if strings.TrimSpace(restaurant.Telephone) == "" {
		restaurant.Telephone = normalizeText(doc.Find("meta[property='restaurant:contact_info:phone_number']").AttrOr("content", ""))
	}
	if strings.TrimSpace(restaurant.Introduction) == "" {
		restaurant.Introduction = normalizeText(doc.Find("meta[name='description']").AttrOr("content", ""))
	}

	// Fallbacks from restaurant information table.
	if strings.TrimSpace(restaurant.OpeningHours) == "" {
		restaurant.OpeningHours = extractTabelogInfoByHeader(doc, []string{"opening hours", "business hours", "営業時間"})
	}
	if strings.TrimSpace(restaurant.Direction) == "" {
		restaurant.Direction = extractTabelogInfoByHeader(doc, []string{"nearest station", "最寄り駅"})
	}
	if strings.TrimSpace(restaurant.PaymentMethod) == "" {
		restaurant.PaymentMethod = extractTabelogInfoByHeader(doc, []string{"payment methods", "支払い方法"})
	}
	if strings.TrimSpace(restaurant.SeatNo) == "" {
		restaurant.SeatNo = extractTabelogInfoByHeader(doc, []string{"number of seats", "席数"})
	}
	if strings.TrimSpace(restaurant.PriceRange) == "" {
		restaurant.PriceRange = extractTabelogInfoByHeader(doc, []string{"budget", "予算"})
	}
	if strings.TrimSpace(restaurant.RestaurantType) == "" {
		restaurant.RestaurantType = extractTabelogInfoByHeader(doc, []string{"categories", "genre", "ジャンル"})
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

func providerDebugURL(provider string, sourceID string, website string) string {
	site := strings.TrimSpace(website)
	if site != "" {
		return site
	}
	id := strings.TrimSpace(sourceID)
	switch NormalizeMarkerWebsiteProviderID(provider) {
	case constant.Openrice:
		return "https://s.openrice.com/" + id
	case constant.Yelp:
		return "https://www.yelp.com/biz/" + url.PathEscape(id)
	case constant.Tabelog:
		return "https://tabelog.com/" + strings.TrimPrefix(id, "/")
	default:
		return id
	}
}

func applyTabelogJSONLD(restaurant *dbmodel.Restaurant, doc *goquery.Document) {
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(i int, s *goquery.Selection) bool {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return true
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return true
		}
		itemType := strings.ToLower(normalizeText(interfaceToString(payload["@type"])))
		if itemType != "restaurant" {
			return true
		}

		if name := normalizeText(interfaceToString(payload["name"])); name != "" {
			restaurant.Name = name
		}
		if image := normalizeText(interfaceToString(payload["image"])); image != "" {
			restaurant.PhotoURL = image
		}
		if priceRange := normalizeText(interfaceToString(payload["priceRange"])); priceRange != "" {
			restaurant.PriceRange = priceRange
		}
		if cuisine := normalizeText(interfaceToString(payload["servesCuisine"])); cuisine != "" {
			restaurant.RestaurantType = cuisine
		}
		if telephone := normalizeText(interfaceToString(payload["telephone"])); telephone != "" {
			restaurant.Telephone = telephone
		}
		if website := normalizeText(interfaceToString(payload["@id"])); website != "" {
			restaurant.Website = website
		}
		if ratingMap, ok := payload["aggregateRating"].(map[string]interface{}); ok {
			ratingValue := normalizeText(interfaceToString(ratingMap["ratingValue"]))
			ratingCount := normalizeText(interfaceToString(ratingMap["ratingCount"]))
			if ratingValue != "" && ratingCount != "" {
				restaurant.Rating = fmt.Sprintf("%s (%s reviews)", ratingValue, ratingCount)
			} else if ratingValue != "" {
				restaurant.Rating = ratingValue
			}
		}
		if addressMap, ok := payload["address"].(map[string]interface{}); ok {
			addressParts := []string{
				normalizeText(interfaceToString(addressMap["streetAddress"])),
				normalizeText(interfaceToString(addressMap["addressLocality"])),
				normalizeText(interfaceToString(addressMap["addressRegion"])),
				normalizeText(interfaceToString(addressMap["postalCode"])),
				normalizeText(interfaceToString(addressMap["addressCountry"])),
			}
			filtered := make([]string, 0, len(addressParts))
			for _, part := range addressParts {
				if part != "" {
					filtered = append(filtered, part)
				}
			}
			restaurant.Address = strings.Join(filtered, ", ")
		}

		return false
	})
}

func extractTabelogInfoByHeader(doc *goquery.Document, headers []string) string {
	targets := make([]string, 0, len(headers))
	for _, item := range headers {
		targets = append(targets, strings.ToLower(strings.TrimSpace(item)))
	}

	value := ""
	doc.Find(".rstinfo-table tr").EachWithBreak(func(i int, row *goquery.Selection) bool {
		headerText := normalizeText(row.Find("th").First().Text())
		lowerHeader := strings.ToLower(headerText)
		for _, target := range targets {
			if strings.Contains(lowerHeader, target) {
				value = normalizeText(row.Find("td").First().Text())
				return false
			}
		}
		return true
	})
	return value
}

func normalizeText(input string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
}

func interfaceToString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

func yelpAttributeBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		lowered := strings.ToLower(strings.TrimSpace(typed))
		return lowered == "true" || lowered == "yes" || lowered == "1"
	default:
		return false
	}
}
