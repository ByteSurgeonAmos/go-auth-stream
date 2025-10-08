package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)


type Platform string

const (
    PlatformX        Platform = "x"
    PlatformFacebook Platform = "facebook"
    PlatformInstagram Platform = "instagram"
)

type Company struct{
	CompanyName string `bson:"company_name" json:"company_name"`
	Link string `bson:"link" json:"link"`
	Socials []Social `bson:"socials" json:"socials"`
	ScrapedData *ScrapedCompanyData `bson:"scraped_data,omitempty" json:"scraped_data,omitempty"`
	LastScraped time.Time `bson:"last_scraped,omitempty" json:"last_scraped,omitempty"`
	ScrapeStatus string `bson:"scrape_status,omitempty" json:"scrape_status,omitempty"` // "success", "failed", "pending"
}

type ScrapedCompanyData struct {
	CompanyID        string            `bson:"company_id" json:"company_id"`
	Description      string            `bson:"description" json:"description"`
	Summary          string            `bson:"summary" json:"summary"`
	RawText          string            `bson:"raw_text" json:"raw_text"`
	Metadata         map[string]string `bson:"metadata" json:"metadata"`
	FaviconURL       string            `bson:"favicon_url" json:"favicon_url"`
	FaviconData      string            `bson:"favicon_data" json:"favicon_data"`
	FaviconContentType string          `bson:"favicon_content_type" json:"favicon_content_type"`
	ScrapedAt        time.Time         `bson:"scraped_at" json:"scraped_at"`
}

type Social struct {
    Platform     Platform  `bson:"platform"`
    Username    string    `bson:"username"`
    URL         string    `bson:"url"`
    AccessToken string    `bson:"access_token"`
    RefreshToken string   `bson:"refresh_token,omitempty"`
    ExpiresAt   time.Time `bson:"expires_at,omitempty"`
}

type User struct{
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserName string `bson:"username" json:"username"`
	Email string	`bson:"email" json:"email"`
	Password string	`bson:"password" json:"-"`
	Companies []Company `bson:"company" json:"company"`
	TwoFactorEnabled bool `bson:"two_factor_enabled" json:"two_factor_enabled"`
	TwoFactorCode string `bson:"two_factor_code,omitempty" json:"-"`
	TwoFactorExpiry time.Time `bson:"two_factor_expiry,omitempty" json:"-"`
}

