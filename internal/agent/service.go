package agent

import (
	"context"
	"fmt"

	agentpb "github.com/ByteSurgeonAmos/go-auth-stream/proto/github.com/ByteSurgeonAmos/go-auth-stream/proto/agent"
)

type Service struct {
	client *Client
}

func NewService() (*Service, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent client: %w", err)
	}
	
	return &Service{client: client}, nil
}

func (s *Service) Close() error {
	return s.client.Close()
}


func (s *Service) GeneratePostForUser(ctx context.Context, userID, platform string, opts ...PostOption) (*agentpb.GeneratePostResponse, error) {
	req := &agentpb.GeneratePostRequest{
		UserId:   userID,
		Platform: platform,
	}
	
	for _, opt := range opts {
		opt(req)
	}
	
	return s.client.GeneratePost(ctx, req)
}

func (s *Service) GetCompaniesForUser(ctx context.Context, userID string) (*agentpb.GetUserCompaniesResponse, error) {
	req := &agentpb.GetUserCompaniesRequest{
		UserId: userID,
	}
	
	return s.client.GetUserCompanies(ctx, req)
}

func (s *Service) CreateUserConversation(ctx context.Context, userID string, opts ...ConversationOption) (*agentpb.ConversationResponse, error) {
	req := &agentpb.CreateConversationRequest{
		UserId: userID,
	}
	
	for _, opt := range opts {
		opt(req)
	}
	
	return s.client.CreateConversation(ctx, req)
}

func (s *Service) GetUserConversationsWithLimit(ctx context.Context, userID string, limit *int32) (*agentpb.GetUserConversationsResponse, error) {
	req := &agentpb.GetUserConversationsRequest{
		UserId: userID,
		Limit:  limit,
	}
	
	return s.client.GetUserConversations(ctx, req)
}

type PostOption func(*agentpb.GeneratePostRequest)

func WithConversationID(conversationID string) PostOption {
	return func(req *agentpb.GeneratePostRequest) {
		req.ConversationId = &conversationID
	}
}

func WithCompanyID(companyID string) PostOption {
	return func(req *agentpb.GeneratePostRequest) {
		req.CompanyId = &companyID
	}
}

func WithCompanyURL(companyURL string) PostOption {
	return func(req *agentpb.GeneratePostRequest) {
		req.CompanyUrl = &companyURL
	}
}

func WithCustomPrompt(prompt string) PostOption {
	return func(req *agentpb.GeneratePostRequest) {
		req.CustomPrompt = &prompt
	}
}

func WithIncludeCompanyData(include bool) PostOption {
	return func(req *agentpb.GeneratePostRequest) {
		req.IncludeCompanyData = &include
	}
}

type ConversationOption func(*agentpb.CreateConversationRequest)

func WithTitle(title string) ConversationOption {
	return func(req *agentpb.CreateConversationRequest) {
		req.Title = &title
	}
}

func WithInitialMessage(message string) ConversationOption {
	return func(req *agentpb.CreateConversationRequest) {
		req.InitialMessage = &message
	}
}

func WithContext(context string) ConversationOption {
	return func(req *agentpb.CreateConversationRequest) {
		req.Context = &context
	}
}

func (s *Service) GetClient() *Client {
	return s.client
}