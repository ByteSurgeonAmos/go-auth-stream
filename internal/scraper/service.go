package scraper

import (
	"fmt"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
)

type Service struct {
	client *Client
}

func NewService() (*Service, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create scraper client: %w", err)
	}
	
	return &Service{client: client}, nil
}

func (s *Service) Close() error {
	return s.client.Close()
}

func (s *Service) ScrapeCompany(url, companyName string) (*models.ScrapedCompanyData, error) {
	if url == "" {
		return nil, fmt.Errorf("URL is required")
	}
	
	ctx, cancel := utils.TimeoutWindow(30)
	defer cancel()
	
	requestID := fmt.Sprintf("%d_%s", time.Now().UnixNano(), companyName)
	companyData, err := s.client.ScrapeAndGetData(ctx, url, requestID)
	if err != nil {
		return nil, err
	}
	
	return &models.ScrapedCompanyData{
		CompanyID:          companyData.Id,
		Description:        companyData.Description,
		Summary:            companyData.Summary,
		RawText:            companyData.RawText,
		Metadata:           companyData.Metadata,
		FaviconURL:         companyData.FaviconUrl,
		FaviconData:        companyData.FaviconData,
		FaviconContentType: companyData.FaviconContentType,
		ScrapedAt:          time.Now(),
	}, nil
}