package models

import (
	"encoding/json"
	"net/url"

	"github.com/shopspring/decimal"
)

type Location struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Area struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LocationID string `json:"location_id"`
}

type CommodityType string

const (
	CommodityTypeWhiteGoods  CommodityType = "white_goods"
	CommodityTypeElectronics CommodityType = "electronics"
	CommodityTypeFurniture   CommodityType = "furniture"
	CommodityTypeClothes     CommodityType = "clothes"
	CommodyTypeOther         CommodityType = "other"
)

type CommodityStatus string

const (
	CommodityStatusInUse      CommodityStatus = "in_use"
	CommodityStatusSold       CommodityStatus = "sold"
	CommodityStatusLost       CommodityStatus = "lost"
	CommodityStatusDisposed   CommodityStatus = "disposed"
	CommodityStatusWrittenOff CommodityStatus = "written_off"
)

type Currency string

type URL url.URL

func (u *URL) MarshalJSON() ([]byte, error) {
	tmp := (*url.URL)(u)
	return json.Marshal(tmp.String())
}

func (u *URL) UnmarshalJSON(data []byte) error {
	var tmp string
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	v, err := url.Parse(tmp)
	if err != nil {
		return err
	}

	*u = URL(*v)
	return nil
}

type Commodity struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	ShortName              string          `json:"short_name"`
	URLs                   []URL           `json:"urls"`
	Type                   CommodityType   `json:"type"`
	AreaID                 string          `json:"area_id"`
	Count                  int             `json:"count"`
	OriginalPrice          decimal.Decimal `json:"original_price"`
	OriginalPriceCurrency  Currency        `json:"original_price_currency"`
	ConvertedOriginalPrice decimal.Decimal `json:"converted_original_price"`
	CurrentPrice           decimal.Decimal `json:"current_price"`
	SerialNumber           string          `json:"serial_number"`
	ExtraSerialNumbers     []string        `json:"extra_serial_numbers"`
	PartNumbers            []string        `json:"part_numbers"`
	Tags                   []string        `json:"tags"`
	ImageIDs               []string        `json:"image_ids"`
	ManualIDs              []string        `json:"manual_ids"`
	Invoice                Invoice         `json:"invoice"`
	Status                 CommodityStatus `json:"status"`
	PurchaseDate           string          `json:"purchase_date"`
	RegisteredDate         string          `json:"registered_date"`
	LastModifiedDate       string          `json:"last_modified_date"`
	Comments               string          `json:"comments"`
	Draft                  bool            `json:"draft"`
}

type Image struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	CommodityID string `json:"commodity_id"`
}

type Manual struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	CommodityID string `json:"commodity_id"`
}

type Invoice struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}
