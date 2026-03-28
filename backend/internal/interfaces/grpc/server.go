// Package grpc provides the gRPC server implementation for Cortex.
package grpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
)

// Server is the gRPC server for Cortex.
type Server struct {
	config    *config.Config
	logger    zerolog.Logger
	server    *grpc.Server
	health    *health.Server
	publisher event.Publisher
	listener  net.Listener

	// Service implementations
	adminService       cortexv1.AdminServiceServer
	fileService        cortexv1.FileServiceServer
	metaService        cortexv1.MetadataServiceServer
	llmService         cortexv1.LLMServiceServer
	ragService         cortexv1.RAGServiceServer
	knowledgeService   cortexv1.KnowledgeServiceServer
	entityService      cortexv1.EntityServiceServer
	sfsService         cortexv1.SemanticFileSystemServiceServer
	clusteringService  cortexv1.ClusteringServiceServer
	taxonomyService    cortexv1.TaxonomyServiceServer
	preferencesService cortexv1.PreferencesServiceServer

	mu       sync.Mutex
	running  bool
	shutdown chan struct{}
}

// NewServer creates a new gRPC server.
func NewServer(cfg *config.Config, logger zerolog.Logger, publisher event.Publisher) *Server {
	return &Server{
		config:    cfg,
		logger:    logger.With().Str("component", "grpc").Logger(),
		publisher: publisher,
		shutdown:  make(chan struct{}),
	}
}

// Start starts the gRPC server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	// Create listener
	listener, err := net.Listen("tcp", s.config.GRPCAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.GRPCAddress, err)
	}
	s.listener = listener

	// Create gRPC server with options
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      2 * time.Hour,      // Extended for long-running streams
			MaxConnectionAgeGrace: 10 * time.Second,    // Extended grace period
			Time:                  5 * time.Minute,
			Timeout:               10 * time.Second,    // Extended timeout for keepalive
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,     // Reduced minimum time between pings
			PermitWithoutStream: true,
		}),
		grpc.ChainUnaryInterceptor(
			s.loggingInterceptor,
			s.recoveryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			s.streamLoggingInterceptor,
			s.streamRecoveryInterceptor,
		),
	}

	s.server = grpc.NewServer(opts...)

	// Register health service
	s.health = health.NewServer()
	healthpb.RegisterHealthServer(s.server, s.health)
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Enable reflection for debugging
	reflection.Register(s.server)

	// Register service handlers
	s.registerServices()

	s.logger.Info().
		Str("address", s.config.GRPCAddress).
		Msg("Starting gRPC server")

	// Start serving in goroutine
	go func() {
		if err := s.server.Serve(listener); err != nil {
			s.logger.Error().Err(err).Msg("gRPC server error")
		}
	}()

	return nil
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info().Msg("Stopping gRPC server")

	// Mark as not serving
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	// Graceful stop with timeout
	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		s.logger.Info().Msg("gRPC server stopped gracefully")
	case <-ctx.Done():
		s.logger.Warn().Msg("Forcing gRPC server stop")
		s.server.Stop()
	}

	s.running = false
	close(s.shutdown)

	return nil
}

// Address returns the server's listening address.
func (s *Server) Address() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.GRPCAddress
}

// RegisterAdminService registers the admin service implementation.
func (s *Server) RegisterAdminService(svc cortexv1.AdminServiceServer) {
	s.adminService = svc
}

// RegisterFileService registers the file service implementation.
func (s *Server) RegisterFileService(svc cortexv1.FileServiceServer) {
	s.fileService = svc
}

// RegisterMetadataService registers the metadata service implementation.
func (s *Server) RegisterMetadataService(svc cortexv1.MetadataServiceServer) {
	s.metaService = svc
}

// RegisterLLMService registers the LLM service implementation.
func (s *Server) RegisterLLMService(svc cortexv1.LLMServiceServer) {
	s.llmService = svc
}

// RegisterRAGService registers the RAG service implementation.
func (s *Server) RegisterRAGService(svc cortexv1.RAGServiceServer) {
	s.ragService = svc
}

// RegisterKnowledgeService registers the knowledge service implementation.
func (s *Server) RegisterKnowledgeService(svc cortexv1.KnowledgeServiceServer) {
	s.knowledgeService = svc
}

// RegisterEntityService registers the entity service implementation.
func (s *Server) RegisterEntityService(svc cortexv1.EntityServiceServer) {
	s.entityService = svc
}

// RegisterSFSService registers the semantic file system service implementation.
func (s *Server) RegisterSFSService(svc cortexv1.SemanticFileSystemServiceServer) {
	s.sfsService = svc
}

// RegisterClusteringService registers the clustering service implementation.
func (s *Server) RegisterClusteringService(svc cortexv1.ClusteringServiceServer) {
	s.clusteringService = svc
}

// RegisterTaxonomyService registers the taxonomy service implementation.
func (s *Server) RegisterTaxonomyService(svc cortexv1.TaxonomyServiceServer) {
	s.taxonomyService = svc
}

// RegisterPreferencesService registers the preferences service implementation.
func (s *Server) RegisterPreferencesService(svc cortexv1.PreferencesServiceServer) {
	s.preferencesService = svc
}

// registerServices registers all gRPC service implementations.
func (s *Server) registerServices() {
	if s.adminService != nil {
		cortexv1.RegisterAdminServiceServer(s.server, s.adminService)
		s.logger.Debug().Msg("AdminService registered")
	}
	if s.fileService != nil {
		cortexv1.RegisterFileServiceServer(s.server, s.fileService)
		s.logger.Debug().Msg("FileService registered")
	}
	if s.metaService != nil {
		cortexv1.RegisterMetadataServiceServer(s.server, s.metaService)
		s.logger.Debug().Msg("MetadataService registered")
	}
	if s.llmService != nil {
		cortexv1.RegisterLLMServiceServer(s.server, s.llmService)
		s.logger.Debug().Msg("LLMService registered")
	}
	if s.ragService != nil {
		cortexv1.RegisterRAGServiceServer(s.server, s.ragService)
		s.logger.Debug().Msg("RAGService registered")
	}
	if s.knowledgeService != nil {
		cortexv1.RegisterKnowledgeServiceServer(s.server, s.knowledgeService)
		s.logger.Debug().Msg("KnowledgeService registered")
	}
	if s.entityService != nil {
		cortexv1.RegisterEntityServiceServer(s.server, s.entityService)
		s.logger.Debug().Msg("EntityService registered")
	}
	if s.sfsService != nil {
		cortexv1.RegisterSemanticFileSystemServiceServer(s.server, s.sfsService)
		s.logger.Debug().Msg("SemanticFileSystemService registered")
	}
	if s.clusteringService != nil {
		cortexv1.RegisterClusteringServiceServer(s.server, s.clusteringService)
		s.logger.Debug().Msg("ClusteringService registered")
	}
	if s.taxonomyService != nil {
		cortexv1.RegisterTaxonomyServiceServer(s.server, s.taxonomyService)
		s.logger.Debug().Msg("TaxonomyService registered")
	}
	if s.preferencesService != nil {
		cortexv1.RegisterPreferencesServiceServer(s.server, s.preferencesService)
		s.logger.Debug().Msg("PreferencesService registered")
	}

	s.logger.Debug().Msg("gRPC services registered")
}

// loggingInterceptor logs unary RPC calls.
func (s *Server) loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	logger := s.logger.With().
		Str("method", info.FullMethod).
		Dur("duration", duration).
		Logger()

	if err != nil {
		// Check if this is a NotFound error - these are expected in several scenarios:
		// 1. RAG finds documents that don't have corresponding files in the index
		// 2. Files that haven't been fully processed yet (metadata not created)
		// 3. Temporary inconsistencies during indexing
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			// For GetFile and GetMetadata NotFound errors, log as debug since they're expected
			// when documents exist but files/metadata don't (data inconsistency or path encoding mismatch)
			if strings.Contains(info.FullMethod, "GetFile") {
				logger.Debug().Err(err).Msg("RPC failed (file not found - expected when documents exist but files don't)")
			} else if strings.Contains(info.FullMethod, "GetMetadata") {
				logger.Debug().Err(err).Msg("RPC failed (metadata not found - expected when files haven't been fully processed)")
			} else {
				logger.Warn().Err(err).Msg("RPC failed (not found)")
			}
		} else {
			logger.Error().Err(err).Msg("RPC failed")
		}
	} else {
		logger.Debug().Msg("RPC completed")
	}

	return resp, err
}

// recoveryInterceptor recovers from panics in unary handlers.
func (s *Server) recoveryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().
				Str("method", info.FullMethod).
				Interface("panic", r).
				Msg("Recovered from panic in RPC handler")
			err = fmt.Errorf("internal server error")
		}
	}()
	return handler(ctx, req)
}

// streamLoggingInterceptor logs streaming RPC calls.
func (s *Server) streamLoggingInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()

	err := handler(srv, ss)

	duration := time.Since(start)
	logger := s.logger.With().
		Str("method", info.FullMethod).
		Dur("duration", duration).
		Logger()

	if err != nil {
		// Check if this is a normal client cancellation (not an error)
		if isNormalStreamClose(err) {
			logger.Debug().Err(err).Msg("Stream closed by client")
		} else {
			logger.Error().Err(err).Msg("Stream RPC failed")
		}
	} else {
		logger.Debug().Msg("Stream RPC completed")
	}

	return err
}

// isNormalStreamClose checks if an error represents a normal stream closure.
func isNormalStreamClose(err error) bool {
	if err == nil {
		return false
	}
	
	// Check gRPC status codes for normal cancellations
	if st, ok := status.FromError(err); ok {
		code := st.Code()
		// Canceled and Unavailable (transport closing) are normal client disconnections
		if code == codes.Canceled || code == codes.Unavailable {
			return true
		}
	}
	
	// Also check error message for common patterns
	errStr := strings.ToLower(err.Error())
	normalClosures := []string{
		"context canceled",
		"transport is closing",
		"eof",
	}
	
	for _, closure := range normalClosures {
		if strings.Contains(errStr, closure) {
			return true
		}
	}
	return false
}

// streamRecoveryInterceptor recovers from panics in stream handlers.
func (s *Server) streamRecoveryInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().
				Str("method", info.FullMethod).
				Interface("panic", r).
				Msg("Recovered from panic in stream RPC handler")
			err = fmt.Errorf("internal server error")
		}
	}()
	return handler(srv, ss)
}
