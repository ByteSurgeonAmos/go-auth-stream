package handlers

import (
	"log"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/internal/scraper"
	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
)

var scraperService *scraper.Service

func InitScraperHandler() error {
	service, err := scraper.NewService()
	if err != nil {
		return err
	}
	scraperService = service
	return nil
}

func CloseScraperHandler() error {
	if scraperService != nil {
		return scraperService.Close()
	}
	return nil
}

func scrapeCompanyAsync(company *models.Company, userID string, companyIndex int) {
	go func() {
		log.Printf("Starting scraping for company: %s", company.CompanyName)
		
		company.ScrapeStatus = "pending"
		company.LastScraped = time.Now()
		
		scrapedData, err := scraperService.ScrapeCompany(company.Link, company.CompanyName)
		if err != nil {
			log.Printf("Failed to scrape company %s: %v", company.CompanyName, err)
			company.ScrapeStatus = "failed"
		} else {
			log.Printf("Successfully scraped company: %s", company.CompanyName)
			company.ScrapeStatus = "success"
			company.ScrapedData = scrapedData
		}
		
		err = updateCompanyScrapedData(userID, companyIndex, company)
		if err != nil {
			log.Printf("Failed to update scraped data for company %s: %v", company.CompanyName, err)
		}
	}()
}

func updateCompanyScrapedData(userID string, companyIndex int, company *models.Company) error {
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()
	
	filter := map[string]interface{}{"_id": userID}
	update := map[string]interface{}{
		"$set": map[string]interface{}{
			"company." + string(rune('0'+companyIndex)) + ".scraped_data":  company.ScrapedData,
			"company." + string(rune('0'+companyIndex)) + ".last_scraped": company.LastScraped,
			"company." + string(rune('0'+companyIndex)) + ".scrape_status": company.ScrapeStatus,
		},
	}
	
	_, err := usersCollection.UpdateOne(ctx, filter, update)
	return err
}