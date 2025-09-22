package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/internal/agent"
	agentpb "github.com/ByteSurgeonAmos/go-auth-stream/proto/github.com/ByteSurgeonAmos/go-auth-stream/proto/agent"
	"github.com/gin-gonic/gin"
)

var agentService *agent.Service

func InitAgentHandler() error {
	service, err := agent.NewService()
	if err != nil {
		return err
	}
	agentService = service
	return nil
}

func CloseAgentHandler() error {
	if agentService != nil {
		return agentService.Close()
	}
	return nil
}

func StreamPostGeneration(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	var req GeneratePostStreamRequest
	var err error

	if c.Request.Method == "GET" {
		req.UserID = c.Query("user_id")
		req.Platform = c.Query("platform")
		
		if conversationID := c.Query("conversation_id"); conversationID != "" {
			req.ConversationID = &conversationID
		}
		if companyID := c.Query("company_id"); companyID != "" {
			req.CompanyID = &companyID
		}
		if companyURL := c.Query("company_url"); companyURL != "" {
			req.CompanyURL = &companyURL
		}
		if customPrompt := c.Query("custom_prompt"); customPrompt != "" {
			req.CustomPrompt = &customPrompt
		}
		if includeCompanyData := c.Query("include_company_data"); includeCompanyData == "true" {
			include := true
			req.IncludeCompanyData = &include
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			sendSSEError(c, "Invalid request format", err)
			return
		}
	}

	if req.UserID == "" || req.Platform == "" {
		sendSSEError(c, "Missing required fields: user_id and platform", nil)
		return
	}

	log.Printf("Starting stream for user: %s, platform: %s", req.UserID, req.Platform)

	if agentService == nil {
		sendSSEError(c, "Agent service not available", nil)
		return
	}

	sendSSEEvent(c, "connected", map[string]interface{}{
		"message": "Post generation started",
		"user_id": req.UserID,
	})
	
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	grpcReq := &agentpb.GeneratePostRequest{
		UserId:   req.UserID,
		Platform: req.Platform,
	}

	if req.ConversationID != nil {
		grpcReq.ConversationId = req.ConversationID
	}
	if req.CompanyID != nil {
		grpcReq.CompanyId = req.CompanyID
	}
	if req.CompanyURL != nil {
		grpcReq.CompanyUrl = req.CompanyURL
	}
	if req.CustomPrompt != nil {
		grpcReq.CustomPrompt = req.CustomPrompt
	}
	if req.IncludeCompanyData != nil {
		grpcReq.IncludeCompanyData = req.IncludeCompanyData
	}

	stream, err := agentService.GetClient().GeneratePostStream(ctx, grpcReq)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "Unimplemented") || 
		   strings.Contains(errStr, "not implement") || 
		   strings.Contains(errStr, "does not implement") {
			log.Printf("Streaming not implemented, falling back to regular generation: %v", err)
			streamWithRegularGeneration(c, grpcReq, req.UserID, ctx)
			return
		}
		
		log.Printf("Failed to start gRPC stream: %v", err)
		sendSSEError(c, "Failed to start post generation stream", err)
		return
	}

	log.Printf("gRPC stream started successfully")

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			sendSSEEvent(c, "completed", map[string]interface{}{
				"message": "Post generation completed",
			})
			log.Printf("Stream completed for user: %s", req.UserID)
			break
		}
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "Unimplemented") || 
			   strings.Contains(errStr, "not implement") || 
			   strings.Contains(errStr, "does not implement") {
				log.Printf("Stream recv error - method not implemented, falling back: %v", err)
				streamWithRegularGeneration(c, grpcReq, req.UserID, ctx)
				return
			}
			
			log.Printf("Stream error: %v", err)
			sendSSEError(c, "Stream error", err)
			break
		}

		eventData := convertGRPCEventToSSE(event)
		eventType := getEventType(event.Type)
		
		log.Printf("Received event: %s for user: %s", eventType, req.UserID)
		sendSSEEvent(c, eventType, eventData)

		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}

		if event.Type == agentpb.PostGenerationEvent_GENERATION_COMPLETED ||
			event.Type == agentpb.PostGenerationEvent_GENERATION_ERROR {
			break
		}
	}
}

func streamWithRegularGeneration(c *gin.Context, grpcReq *agentpb.GeneratePostRequest, userID string, ctx context.Context) {
	log.Printf("Using fallback generation for user: %s", userID)
	
	sendSSEEvent(c, "started", map[string]interface{}{
		"type":       "GENERATION_STARTED",
		"user_id":    userID,
		"request_id": fmt.Sprintf("req_%d", time.Now().Unix()),
		"timestamp":  time.Now().Unix(),
		"data":       "Starting post generation...",
	})
	
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	time.Sleep(500 * time.Millisecond)
	sendSSEEvent(c, "progress", map[string]interface{}{
		"type":       "GENERATION_PROGRESS",
		"user_id":    userID,
		"request_id": fmt.Sprintf("req_%d", time.Now().Unix()),
		"timestamp":  time.Now().Unix(),
		"data":       "Analyzing requirements...",
	})
	
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	time.Sleep(500 * time.Millisecond)
	sendSSEEvent(c, "progress", map[string]interface{}{
		"type":       "GENERATION_PROGRESS",
		"user_id":    userID,
		"request_id": fmt.Sprintf("req_%d", time.Now().Unix()),
		"timestamp":  time.Now().Unix(),
		"data":       "Generating content...",
	})
	
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	log.Printf("Sending GeneratePost request: userId=%s, platform=%s, conversationId=%v", 
		grpcReq.UserId, grpcReq.Platform, grpcReq.ConversationId)

	response, err := agentService.GetClient().GeneratePost(ctx, grpcReq)
	if err != nil {
		log.Printf("Failed to generate post: %v", err)
		
		if strings.Contains(err.Error(), "userId") || strings.Contains(err.Error(), "validation") {
			log.Printf("Detected validation error, this might be a conversation creation issue in the agent service")
			sendSSEError(c, "Validation error in agent service - check that userId is properly passed for conversation creation", err)
		} else {
			sendSSEError(c, "Failed to generate post", err)
		}
		return
	}


	
	if response != nil {
		log.Printf("ConversationId: %s", response.ConversationId)
		if response.PostResponse != nil {
			log.Printf("PostResponse exists:")
			log.Printf("  PostId: %s", response.PostResponse.PostId)
			log.Printf("  Content: %s", response.PostResponse.Content)
			log.Printf("  Platform: %s", response.PostResponse.Platform)
			log.Printf("  Hashtags: %v", response.PostResponse.Hashtags)
			if response.PostResponse.Metadata != nil {
				log.Printf("  Metadata: %+v", response.PostResponse.Metadata)
			} else {
				log.Printf("  Metadata: nil")
			}
		} else {
			log.Printf("PostResponse is nil - THIS IS THE PROBLEM!")
		}
	} else {
		log.Printf("Response is completely nil!")
	}
	log.Printf("=== END RESPONSE DEBUG ===")
	
	completionData := map[string]interface{}{
		"type":         "GENERATION_COMPLETED",
		"user_id":      userID,
		"request_id":   fmt.Sprintf("req_%d", time.Now().Unix()),
		"timestamp":    time.Now().Unix(),
		"message":      "Post generation completed successfully",
		"response":     response,
	}

	if response != nil && response.PostResponse != nil {
		completionData["post"] = map[string]interface{}{
			"post_id":     response.PostResponse.PostId,
			"content":     response.PostResponse.Content,
			"hashtags":    response.PostResponse.Hashtags,
			"platform":    response.PostResponse.Platform,
			"metadata":    response.PostResponse.Metadata,
		}
		completionData["conversation_id"] = response.ConversationId
		log.Printf("Added post data to completion event")
	} else {
		log.Printf("No PostResponse data available - sending empty response to client")
		completionData["debug_info"] = "PostResponse was nil from agent service"
	}

	sendSSEEvent(c, "completed", completionData)
	
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	log.Printf("Post generation completed successfully for user: %s", userID)
}

func GeneratePost(c *gin.Context) {
	var req GeneratePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if req.UserID == "" || req.Platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields: user_id and platform"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	options := []agent.PostOption{}
	if req.ConversationID != nil {
		options = append(options, agent.WithConversationID(*req.ConversationID))
	}
	if req.CompanyID != nil {
		options = append(options, agent.WithCompanyID(*req.CompanyID))
	}
	if req.CompanyURL != nil {
		options = append(options, agent.WithCompanyURL(*req.CompanyURL))
	}
	if req.CustomPrompt != nil {
		options = append(options, agent.WithCustomPrompt(*req.CustomPrompt))
	}
	if req.IncludeCompanyData != nil {
		options = append(options, agent.WithIncludeCompanyData(*req.IncludeCompanyData))
	}

	response, err := agentService.GeneratePostForUser(ctx, req.UserID, req.Platform, options...)
	if err != nil {
		log.Printf("Failed to generate post: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate post"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func sendSSEEvent(c *gin.Context, eventType string, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(c.Writer, "event: %s\n", eventType)
	fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
}

func sendSSEError(c *gin.Context, message string, err error) {
	errorData := map[string]interface{}{
		"error":   message,
		"details": "",
	}
	if err != nil {
		errorData["details"] = err.Error()
	}
	sendSSEEvent(c, "error", errorData)
}

func convertGRPCEventToSSE(event *agentpb.PostGenerationEvent) map[string]interface{} {
	data := map[string]interface{}{
		"type":       event.Type.String(),
		"user_id":    event.UserId,
		"request_id": event.RequestId,
		"timestamp":  event.Timestamp,
	}

	if event.Data != nil {
		data["data"] = *event.Data
	}
	if event.Error != nil {
		data["error"] = *event.Error
	}

	return data
}

func getEventType(eventType agentpb.PostGenerationEvent_EventType) string {
	switch eventType {
	case agentpb.PostGenerationEvent_GENERATION_STARTED:
		return "started"
	case agentpb.PostGenerationEvent_GENERATION_PROGRESS:
		return "progress"
	case agentpb.PostGenerationEvent_GENERATION_COMPLETED:
		return "completed"
	case agentpb.PostGenerationEvent_GENERATION_ERROR:
		return "error"
	default:
		return "unknown"
	}
}

type GeneratePostRequest struct {
	UserID             string  `json:"user_id" binding:"required"`
	ConversationID     *string `json:"conversation_id,omitempty"`
	Platform           string  `json:"platform" binding:"required"`
	CompanyID          *string `json:"company_id,omitempty"`
	CompanyURL         *string `json:"company_url,omitempty"`
	CustomPrompt       *string `json:"custom_prompt,omitempty"`
	IncludeCompanyData *bool   `json:"include_company_data,omitempty"`
}

type GeneratePostStreamRequest struct {
	GeneratePostRequest
}

func GetUserCompanies(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id parameter"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := agentService.GetCompaniesForUser(ctx, userID)
	if err != nil {
		log.Printf("Failed to get user companies: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get companies"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required field: user_id"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	options := []agent.ConversationOption{}
	if req.Title != nil {
		options = append(options, agent.WithTitle(*req.Title))
	}
	if req.InitialMessage != nil {
		options = append(options, agent.WithInitialMessage(*req.InitialMessage))
	}
	if req.Context != nil {
		options = append(options, agent.WithContext(*req.Context))
	}

	response, err := agentService.CreateUserConversation(ctx, req.UserID, options...)
	if err != nil {
		log.Printf("Failed to create conversation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversation"})
		return
	}

	c.JSON(http.StatusOK, response)
}

type CreateConversationRequest struct {
	UserID         string  `json:"user_id" binding:"required"`
	Title          *string `json:"title,omitempty"`
	InitialMessage *string `json:"initial_message,omitempty"`
	Context        *string `json:"context,omitempty"`
}