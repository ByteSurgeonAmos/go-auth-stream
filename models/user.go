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
}

