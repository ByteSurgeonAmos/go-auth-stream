package scraper

import (
	"context"
	"fmt"
	"os"
	"time"

	proto "github.com/ByteSurgeonAmos/go-auth-stream/proto/github.com/ByteSurgeonAmos/go-auth-stream/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client proto.ScraperClient
}

func NewClient() (*Client, error) {
	host := os.Getenv("SCRAPPER_HOST")
	port := os.Getenv("SCRAPPER_PORT")
	
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "50051"
	}
	
	address := fmt.Sprintf("%s:%s", host, port)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	conn, err := grpc.DialContext(ctx, address, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to scraper service at %s: %w", address, err)
	}
	
	return &Client{
		conn:   conn,
		client: proto.NewScraperClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) ScrapeAndGetData(ctx context.Context, url, requestID string) (*proto.CompanyData, error) {
	if url == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}
	
	if requestID == "" {
		requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	
	scrapeReq := &proto.ScrapeRequest{
		Url:       url,
		RequestId: requestID,
	}
	
	scrapeResp, err := c.client.Scrape(ctx, scrapeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape URL %s: %w", url, err)
	}
	
	if !scrapeResp.Success {
		return nil, fmt.Errorf("scraping failed: %s", scrapeResp.Error)
	}
	
	companyReq := &proto.GetCompanyRequest{
		CompanyId: scrapeResp.CompanyId,
	}
	
	companyData, err := c.client.GetCompany(ctx, companyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get company data: %w", err)
	}
	
	return companyData, nil
}

