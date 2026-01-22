package grpcagent

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/grpc/grpcagent/agent"
	"github.com/nais/api/internal/grpc/grpcagent/agent/chat"
	"github.com/nais/api/internal/grpc/grpcagent/agent/rag"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"
)

const (
	maxRAGResults = 5
)

// Server implements the Agent gRPC service.
type Server struct {
	conversations *agent.ConversationStore
	chatClient    chat.StreamingClient
	ragClient     rag.DocumentSearcher
	naisAPIURL    string
	log           logrus.FieldLogger
	protoapi.UnimplementedAgentServer
}

// NewServer creates a new Agent gRPC server.
func NewServer(
	pool *pgxpool.Pool,
	chatClient chat.StreamingClient,
	ragClient rag.DocumentSearcher,
	naisAPIURL string,
	log logrus.FieldLogger,
) *Server {
	return &Server{
		conversations: agent.NewConversationStore(pool),
		chatClient:    chatClient,
		ragClient:     ragClient,
		naisAPIURL:    naisAPIURL,
		log:           log,
	}
}

// Chat implements the streaming chat RPC.
func (s *Server) Chat(req *protoapi.ChatRequest, stream grpc.ServerStreamingServer[protoapi.ChatStreamEvent]) error {
	ctx := stream.Context()
	log := s.log.WithField("method", "Chat")

	// Validate request
	if strings.TrimSpace(req.GetMessage()) == "" {
		return status.Error(codes.InvalidArgument, "message is required")
	}

	// Get user ID from context
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, "authentication required")
	}

	log = log.WithField("user_id", userID)

	// Get or create conversation
	conversationID, err := s.conversations.GetOrCreateConversation(ctx, userID, req.GetConversationId(), req.GetMessage())
	if err != nil {
		log.WithError(err).Error("failed to get or create conversation")
		return status.Error(codes.Internal, "failed to process conversation")
	}

	log = log.WithField("conversation_id", conversationID)

	// Send metadata event
	if err := stream.Send(protoapi.ChatStreamEvent_builder{
		Type:           ptr.To(protoapi.ChatStreamEventType_CHAT_STREAM_EVENT_TYPE_METADATA),
		ConversationId: ptr.To(conversationID.String()),
		MessageId:      ptr.To(uuid.New().String()),
	}.Build()); err != nil {
		return err
	}

	// Perform RAG search
	docs, sources, err := s.searchDocumentation(ctx, req.GetMessage())
	if err != nil {
		log.WithError(err).Warn("RAG search failed, continuing without docs")
		agent.RecordRAGSearch("error", 0, 0)
	} else {
		agent.RecordRAGSearch("success", 0, len(docs))
	}

	// Get authorization header from metadata
	authHeader := getAuthHeaderFromContext(ctx)

	// Build chat context
	var chatCtx *agent.ChatContext
	if req.GetContext() != nil {
		chatCtx = &agent.ChatContext{
			Path: req.GetContext().GetPath(),
			Team: req.GetContext().GetTeam(),
			App:  req.GetContext().GetApp(),
			Env:  req.GetContext().GetEnv(),
		}
	}

	// Build orchestrator and run streaming conversation
	orchestrator := agent.NewOrchestrator(
		s.chatClient,
		s.naisAPIURL,
		authHeader,
		log,
	)

	streamCh, err := orchestrator.RunStream(ctx, req.GetMessage(), chatCtx, docs, conversationID)
	if err != nil {
		log.WithError(err).Error("orchestrator stream failed")
		return status.Error(codes.Internal, "failed to start chat stream")
	}

	var fullContent strings.Builder
	var toolsUsed []agent.ToolUsed

	for event := range streamCh {
		switch event.Type {
		case agent.StreamEventToolStart:
			if err := stream.Send(protoapi.ChatStreamEvent_builder{
				Type:            ptr.To(protoapi.ChatStreamEventType_CHAT_STREAM_EVENT_TYPE_TOOL_START),
				ToolName:        ptr.To(event.ToolName),
				ToolDescription: ptr.To(event.Description),
			}.Build()); err != nil {
				return err
			}

		case agent.StreamEventToolEnd:
			if err := stream.Send(protoapi.ChatStreamEvent_builder{
				Type:            ptr.To(protoapi.ChatStreamEventType_CHAT_STREAM_EVENT_TYPE_TOOL_END),
				ToolName:        ptr.To(event.ToolName),
				ToolDescription: ptr.To(event.Description),
				ToolSuccess:     ptr.To(event.Success),
			}.Build()); err != nil {
				return err
			}
			toolsUsed = append(toolsUsed, agent.ToolUsed{
				Name:        event.ToolName,
				Description: event.Description,
			})

		case agent.StreamEventContent:
			fullContent.WriteString(event.Content)
			if err := stream.Send(protoapi.ChatStreamEvent_builder{
				Type:    ptr.To(protoapi.ChatStreamEventType_CHAT_STREAM_EVENT_TYPE_CONTENT),
				Content: ptr.To(event.Content),
			}.Build()); err != nil {
				return err
			}

		case agent.StreamEventError:
			if err := stream.Send(protoapi.ChatStreamEvent_builder{
				Type:         ptr.To(protoapi.ChatStreamEventType_CHAT_STREAM_EVENT_TYPE_ERROR),
				ErrorCode:    ptr.To("stream_error"),
				ErrorMessage: ptr.To(event.Error.Error()),
			}.Build()); err != nil {
				return err
			}
			return status.Error(codes.Internal, event.Error.Error())
		}
	}

	// Send sources if available
	if len(sources) > 0 {
		protoSources := make([]*protoapi.Source, 0, len(sources))
		for _, src := range sources {
			protoSources = append(protoSources, protoapi.Source_builder{
				Title: ptr.To(src.Title),
				Url:   ptr.To(src.URL),
			}.Build())
		}
		if err := stream.Send(protoapi.ChatStreamEvent_builder{
			Type:    ptr.To(protoapi.ChatStreamEventType_CHAT_STREAM_EVENT_TYPE_SOURCES),
			Sources: protoSources,
		}.Build()); err != nil {
			return err
		}
	}

	// Send done event
	if err := stream.Send(protoapi.ChatStreamEvent_builder{
		Type: ptr.To(protoapi.ChatStreamEventType_CHAT_STREAM_EVENT_TYPE_DONE),
	}.Build()); err != nil {
		return err
	}

	// Store messages asynchronously
	go func() {
		result := &agent.OrchestratorResult{
			Content:   fullContent.String(),
			ToolsUsed: toolsUsed,
			Usage:     &chat.UsageStats{},
		}
		if err := s.conversations.StoreMessages(context.Background(), conversationID, req.GetMessage(), result); err != nil {
			log.WithError(err).Error("failed to store messages")
		}
	}()

	agent.RecordChatRequest("success", true, 0)
	agent.RecordToolIterations(len(toolsUsed))

	return nil
}

// ListConversations returns all conversations for the authenticated user.
func (s *Server) ListConversations(ctx context.Context, req *protoapi.ListConversationsRequest) (*protoapi.ListConversationsResponse, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	conversations, err := s.conversations.ListConversations(ctx, userID)
	if err != nil {
		s.log.WithError(err).Error("failed to list conversations")
		return nil, status.Error(codes.Internal, "failed to list conversations")
	}

	protoConversations := make([]*protoapi.ConversationSummary, 0, len(conversations))
	for _, c := range conversations {
		protoConversations = append(protoConversations, protoapi.ConversationSummary_builder{
			Id:        ptr.To(c.ID.String()),
			Title:     ptr.To(c.Title),
			UpdatedAt: timestamppb.New(c.UpdatedAt),
		}.Build())
	}

	return protoapi.ListConversationsResponse_builder{
		Conversations: protoConversations,
	}.Build(), nil
}

// GetConversation returns a specific conversation with all its messages.
func (s *Server) GetConversation(ctx context.Context, req *protoapi.GetConversationRequest) (*protoapi.GetConversationResponse, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	if req.GetConversationId() == "" {
		return nil, status.Error(codes.InvalidArgument, "conversation_id is required")
	}

	conversationID, err := uuid.Parse(req.GetConversationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid conversation_id")
	}

	conv, err := s.conversations.GetConversation(ctx, userID, conversationID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "conversation not found")
		}
		s.log.WithError(err).Error("failed to get conversation")
		return nil, status.Error(codes.Internal, "failed to get conversation")
	}

	protoMessages := make([]*protoapi.ConversationMessage, 0, len(conv.Messages))
	for _, msg := range conv.Messages {
		protoToolsUsed := make([]*protoapi.ToolUsed, 0, len(msg.ToolsUsed))
		for _, tu := range msg.ToolsUsed {
			protoToolsUsed = append(protoToolsUsed, protoapi.ToolUsed_builder{
				Name:        ptr.To(tu.Name),
				Description: ptr.To(tu.Description),
			}.Build())
		}

		protoMessages = append(protoMessages, protoapi.ConversationMessage_builder{
			Id:        ptr.To(msg.ID.String()),
			Role:      ptr.To(msg.Role),
			Content:   ptr.To(msg.Content),
			ToolsUsed: protoToolsUsed,
			CreatedAt: timestamppb.New(msg.CreatedAt),
		}.Build())
	}

	return protoapi.GetConversationResponse_builder{
		Conversation: protoapi.Conversation_builder{
			Id:        ptr.To(conv.ID.String()),
			Title:     ptr.To(conv.Title),
			Messages:  protoMessages,
			CreatedAt: timestamppb.New(conv.CreatedAt),
			UpdatedAt: timestamppb.New(conv.UpdatedAt),
		}.Build(),
	}.Build(), nil
}

// DeleteConversation deletes a conversation.
func (s *Server) DeleteConversation(ctx context.Context, req *protoapi.DeleteConversationRequest) (*protoapi.DeleteConversationResponse, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	if req.GetConversationId() == "" {
		return nil, status.Error(codes.InvalidArgument, "conversation_id is required")
	}

	conversationID, err := uuid.Parse(req.GetConversationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid conversation_id")
	}

	if err := s.conversations.DeleteConversation(ctx, userID, conversationID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "conversation not found")
		}
		s.log.WithError(err).Error("failed to delete conversation")
		return nil, status.Error(codes.Internal, "failed to delete conversation")
	}

	return protoapi.DeleteConversationResponse_builder{
		Deleted: ptr.To(true),
	}.Build(), nil
}

func (s *Server) searchDocumentation(ctx context.Context, query string) ([]rag.Document, []agent.Source, error) {
	result, err := s.ragClient.Search(ctx, query, &rag.SearchOptions{
		MaxResults: maxRAGResults,
	})
	if err != nil {
		return nil, nil, err
	}

	sources := make([]agent.Source, 0, len(result.Documents))
	for _, doc := range result.Documents {
		sources = append(sources, agent.Source{
			Title: doc.Title,
			URL:   doc.URL,
		})
	}

	return result.Documents, sources, nil
}

// getUserIDFromContext extracts the user ID from the gRPC context.
func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "no user in context")
	}
	return actor.User.GetID(), nil
}

// getAuthHeaderFromContext extracts the Authorization header from gRPC metadata.
func getAuthHeaderFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return ""
	}

	return values[0]
}
