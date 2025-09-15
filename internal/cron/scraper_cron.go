package cron

import (
	"log"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/internal/db"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/scraper"
	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type CronService struct {
	scraperService  *scraper.Service
	usersCollection *mongo.Collection
	ticker          *time.Ticker
	quit            chan bool
}

func NewCronService() (*CronService, error) {
	scraperSvc, err := scraper.NewService()
	if err != nil {
		return nil, err
	}

	return &CronService{
		scraperService:  scraperSvc,
		usersCollection: db.DB.Collection("users"),
		quit:            make(chan bool),
	}, nil
}

func (cs *CronService) Start() {
	cs.ticker = time.NewTicker(240 * time.Hour) 
	
	log.Println("Starting cron service for company data refresh (every 10 days)")
	
	go func() {
		for {
			select {
			case <-cs.ticker.C:
				log.Println("Running scheduled company data refresh...")
				cs.refreshStaleCompanyData()
			case <-cs.quit:
				log.Println("Stopping cron service...")
				return
			}
		}
	}()
	
	go func() {
		time.Sleep(30 * time.Second)
		log.Println("Running initial company data refresh check...")
		cs.refreshStaleCompanyData()
	}()
}

func (cs *CronService) Stop() {
	if cs.ticker != nil {
		cs.ticker.Stop()
	}
	cs.quit <- true
	if cs.scraperService != nil {
		cs.scraperService.Close()
	}
}

func (cs *CronService) refreshStaleCompanyData() {
	ctx, cancel := utils.TimeoutWindow(300) 
	defer cancel()

	cutoffDate := time.Now().AddDate(0, 0, -10) 
	
	filter := bson.M{
		"$or": []bson.M{
			{"company.last_scraped": bson.M{"$lt": cutoffDate}},
			{"company.last_scraped": bson.M{"$exists": false}},
			{"company.scrape_status": "failed"},
		},
	}

	cursor, err := cs.usersCollection.Find(ctx, filter)
	if err != nil {
		log.Printf("Error finding users for data refresh: %v", err)
		return
	}
	defer cursor.Close(ctx)

	refreshCount := 0
	for cursor.Next(ctx) {
		var user models.User
		if err := cursor.Decode(&user); err != nil {
			log.Printf("Error decoding user: %v", err)
			continue
		}

		cs.refreshUserCompanies(&user, cutoffDate)
		refreshCount++
	}

	log.Printf("Completed scheduled refresh. Processed %d users.", refreshCount)
}

func (cs *CronService) refreshUserCompanies(user *models.User, cutoffDate time.Time) {
	for i, company := range user.Companies {
		needsRefresh := company.LastScraped.IsZero() || 
						company.LastScraped.Before(cutoffDate) || 
						company.ScrapeStatus == "failed"
		
		if needsRefresh && company.Link != "" {
			log.Printf("Refreshing data for company: %s", company.CompanyName)
			cs.refreshSingleCompany(user.ID.Hex(), i, &company)
			time.Sleep(2 * time.Second) 
		}
	}
}

func (cs *CronService) refreshSingleCompany(userID string, companyIndex int, company *models.Company) {
	company.ScrapeStatus = "pending"
	company.LastScraped = time.Now()

	scrapedData, err := cs.scraperService.ScrapeCompany(company.Link, company.CompanyName)
	if err != nil {
		log.Printf("Failed to refresh company %s: %v", company.CompanyName, err)
		company.ScrapeStatus = "failed"
	} else {
		log.Printf("Successfully refreshed data for company: %s", company.CompanyName)
		company.ScrapeStatus = "success"
		company.ScrapedData = scrapedData
	}

	cs.updateCompanyInDB(userID, companyIndex, company)
}

func (cs *CronService) updateCompanyInDB(userID string, companyIndex int, company *models.Company) {
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	filter := bson.M{"_id": userID}
	update := bson.M{
		"$set": bson.M{
			"company." + string(rune('0'+companyIndex)) + ".scraped_data":  company.ScrapedData,
			"company." + string(rune('0'+companyIndex)) + ".last_scraped": company.LastScraped,
			"company." + string(rune('0'+companyIndex)) + ".scrape_status": company.ScrapeStatus,
		},
	}

	_, err := cs.usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Failed to update company data in DB for %s: %v", company.CompanyName, err)
	}
}