package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	agentpb "github.com/ByteSurgeonAmos/go-auth-stream/proto/github.com/ByteSurgeonAmos/go-auth-stream/proto/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn                        *grpc.ClientConn
	agentClient                 agentpb.AgentServiceClient
	conversationClient          agentpb.ConversationServiceClient
	userPreferencesClient       agentpb.UserPreferencesServiceClient
}

func NewClient() (*Client, error) {
	host := os.Getenv("AGENT_HOST")
	port := os.Getenv("AGENT_PORT")
	
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "50052"
	}
	
	address := fmt.Sprintf("%s:%s", host, port)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	conn, err := grpc.DialContext(ctx, address, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent1 service at %s: %w", address, err)
	}
	
	return &Client{
		conn:                  conn,
		agentClient:           agentpb.NewAgentServiceClient(conn),
		conversationClient:    agentpb.NewConversationServiceClient(conn),
		userPreferencesClient: agentpb.NewUserPreferencesServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) GeneratePost(ctx context.Context, req *agentpb.GeneratePostRequest) (*agentpb.GeneratePostResponse, error) {
	return c.agentClient.GeneratePost(ctx, req)
}

func (c *Client) GeneratePostStream(ctx context.Context, req *agentpb.GeneratePostRequest) (agentpb.AgentService_GeneratePostStreamClient, error) {
	return c.agentClient.GeneratePostStream(ctx, req)
}

func (c *Client) GetUserCompanies(ctx context.Context, req *agentpb.GetUserCompaniesRequest) (*agentpb.GetUserCompaniesResponse, error) {
	return c.agentClient.GetUserCompanies(ctx, req)
}

func (c *Client) GetCompanyDetails(ctx context.Context, req *agentpb.GetCompanyDetailsRequest) (*agentpb.GetCompanyDetailsResponse, error) {
	return c.agentClient.GetCompanyDetails(ctx, req)
}

func (c *Client) CreateConversation(ctx context.Context, req *agentpb.CreateConversationRequest) (*agentpb.ConversationResponse, error) {
	return c.conversationClient.CreateConversation(ctx, req)
}

func (c *Client) GetConversation(ctx context.Context, req *agentpb.GetConversationRequest) (*agentpb.ConversationResponse, error) {
	return c.conversationClient.GetConversation(ctx, req)
}

func (c *Client) GetUserConversations(ctx context.Context, req *agentpb.GetUserConversationsRequest) (*agentpb.GetUserConversationsResponse, error) {
	return c.conversationClient.GetUserConversations(ctx, req)
}

func (c *Client) AddMessage(ctx context.Context, req *agentpb.AddMessageRequest) (*agentpb.ConversationResponse, error) {
	return c.conversationClient.AddMessage(ctx, req)
}

func (c *Client) UpdateConversationTitle(ctx context.Context, req *agentpb.UpdateConversationTitleRequest) (*agentpb.ConversationResponse, error) {
	return c.conversationClient.UpdateConversationTitle(ctx, req)
}

func (c *Client) DeleteConversation(ctx context.Context, req *agentpb.DeleteConversationRequest) (*agentpb.DeleteConversationResponse, error) {
	return c.conversationClient.DeleteConversation(ctx, req)
}

func (c *Client) GetUserPreferences(ctx context.Context, req *agentpb.GetUserPreferencesRequest) (*agentpb.UserPreferencesResponse, error) {
	return c.userPreferencesClient.GetUserPreferences(ctx, req)
}

func (c *Client) UpdateUserPreferences(ctx context.Context, req *agentpb.UpdateUserPreferencesRequest) (*agentpb.UserPreferencesResponse, error) {
	return c.userPreferencesClient.UpdateUserPreferences(ctx, req)
}

func (c *Client) ResetUserPreferences(ctx context.Context, req *agentpb.ResetUserPreferencesRequest) (*agentpb.UserPreferencesResponse, error) {
	return c.userPreferencesClient.ResetUserPreferences(ctx, req)
}