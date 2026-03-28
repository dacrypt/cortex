package clustering

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// ServiceConfig contains configuration for the clustering service.
type ServiceConfig struct {
	GraphBuilder      GraphBuilderConfig
	CommunityDetector CommunityDetectorConfig
	LLMValidator      LLMValidatorConfig

	// Auto-assignment settings
	AutoAssignThreshold float64 // Minimum confidence to auto-assign (default: 0.7)
	CreateProjectsAuto  bool    // Automatically create projects from clusters (default: true)
}

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		GraphBuilder:        DefaultGraphBuilderConfig(),
		CommunityDetector:   DefaultCommunityDetectorConfig(),
		LLMValidator:        DefaultLLMValidatorConfig(),
		AutoAssignThreshold: 0.7,
		CreateProjectsAuto:  true,
	}
}

// Service provides document clustering capabilities.
type Service struct {
	graphBuilder      *GraphBuilder
	communityDetector *CommunityDetector
	llmValidator      *LLMValidator
	clusterRepo       repository.ClusterRepository
	projectRepo       repository.ProjectRepository
	config            ServiceConfig
	logger            zerolog.Logger
}

// NewService creates a new clustering service.
func NewService(
	graphBuilder *GraphBuilder,
	communityDetector *CommunityDetector,
	llmValidator *LLMValidator,
	clusterRepo repository.ClusterRepository,
	projectRepo repository.ProjectRepository,
	config ServiceConfig,
	logger zerolog.Logger,
) *Service {
	return &Service{
		graphBuilder:      graphBuilder,
		communityDetector: communityDetector,
		llmValidator:      llmValidator,
		clusterRepo:       clusterRepo,
		projectRepo:       projectRepo,
		config:            config,
		logger:            logger.With().Str("component", "clustering_service").Logger(),
	}
}

// ClusteringResult contains the results of a clustering operation.
type ClusteringResult struct {
	Clusters         []*entity.DocumentCluster
	ProjectsCreated  int
	AssignmentsMade  int
	ValidationErrors []string
	Duration         time.Duration
}

// RunClustering performs full clustering pipeline:
// 1. Build document graph
// 2. Detect communities
// 3. Validate with LLM (hybrid approach)
// 4. Create/update clusters
// 5. Auto-assign to projects if configured
func (s *Service) RunClustering(ctx context.Context, workspaceID entity.WorkspaceID, forceRebuild bool) (*ClusteringResult, error) {
	start := time.Now()
	result := &ClusteringResult{
		Clusters:         []*entity.DocumentCluster{},
		ValidationErrors: []string{},
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Bool("force_rebuild", forceRebuild).
		Msg("Starting clustering pipeline")

	// Only clear existing clusters if force rebuild is requested
	// Otherwise, we'll update clusters incrementally
	if forceRebuild {
		s.logger.Info().Msg("Force rebuild enabled, clearing existing clusters")
		if err := s.clusterRepo.ClearAllClusters(ctx, workspaceID); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to clear existing clusters, continuing anyway")
		}
	} else {
		s.logger.Info().Msg("Incremental clustering: will update existing clusters and create new ones as needed")
	}

	// Step 1: Build document graph
	graph, err := s.graphBuilder.BuildGraph(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}

	if graph.NodeCount() < 2 {
		s.logger.Info().Msg("Not enough documents for clustering")
		result.Duration = time.Since(start)
		return result, nil
	}

	// Persist the graph for future use
	if err := s.graphBuilder.PersistGraph(ctx, graph); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to persist graph")
	}

	// Step 2: Detect communities
	communities, err := s.communityDetector.DetectCommunities(ctx, graph)
	if err != nil {
		return nil, fmt.Errorf("failed to detect communities: %w", err)
	}

	if len(communities) == 0 {
		s.logger.Info().Msg("No communities detected")
		result.Duration = time.Since(start)
		return result, nil
	}

	s.logger.Info().
		Int("communities", len(communities)).
		Msg("Communities detected")

	// Step 3: Convert to clusters and validate with LLM
	clusters := s.communityDetector.ConvertToDocumentClusters(ctx, workspaceID, communities)
	communityByClusterID := make(map[entity.ClusterID]*Community, len(clusters))
	for i, cluster := range clusters {
		if i < len(communities) {
			communityByClusterID[cluster.ID] = communities[i]
		}
	}

	// Step 4: LLM validation and naming (hybrid approach)
	for i, cluster := range clusters {
		community := communities[i]

		// Find central nodes
		centralNodes := s.communityDetector.FindCentralNodes(graph, community.Members, 3)
		cluster.CentralNodes = centralNodes

		// LLM validation and naming
		// Always generate names with LLM for ALL clusters, regardless of validation result
		if s.llmValidator != nil {
			// Step 1: Try to validate the cluster (optional - for confidence scoring)
			validated := false
			validationErr := error(nil)
			if s.llmValidator != nil {
				validated, validationErr = s.llmValidator.ValidateCluster(ctx, cluster, community.Members, graph)
				if validationErr != nil {
					s.logger.Warn().
						Err(validationErr).
						Str("cluster_id", cluster.ID.String()).
						Msg("LLM validation failed, but will still generate name")
					result.ValidationErrors = append(result.ValidationErrors, validationErr.Error())
				}
			}

			// Step 2: ALWAYS generate cluster metadata (name, summary, keywords) with LLM
			// This ensures ALL clusters get meaningful names, even if validation fails
			if err := s.llmValidator.GenerateClusterMetadata(ctx, cluster, community.Members); err != nil {
				s.logger.Warn().
					Err(err).
					Str("cluster_id", cluster.ID.String()).
					Msg("Failed to generate cluster metadata with LLM, using fallback name")
				// Fallback to generated name based on ID for uniqueness
				if cluster.Name == "" {
					cluster.Name = fmt.Sprintf("Cluster %s", cluster.ID.String()[:8])
				}
				if cluster.Summary == "" {
					cluster.Summary = fmt.Sprintf("Cluster of %d related documents", len(community.Members))
				}
			} else {
				// LLM successfully generated metadata
				// Ensure name is not empty (fallback if LLM didn't provide one)
				if cluster.Name == "" {
					s.logger.Warn().
						Str("cluster_id", cluster.ID.String()).
						Msg("LLM generated metadata but name is empty, using fallback")
					cluster.Name = fmt.Sprintf("Cluster %s", cluster.ID.String()[:8])
				}
				if cluster.Summary == "" {
					cluster.Summary = fmt.Sprintf("Cluster of %d related documents", len(community.Members))
				}
			}

			// Step 3: Adjust confidence based on validation result (but still activate cluster)
			if !validated {
				// Lower confidence for non-validated clusters, but still activate them
				if cluster.Confidence > 0.5 {
					cluster.Confidence = 0.5
				}
				s.logger.Debug().
					Str("cluster_id", cluster.ID.String()).
					Str("name", cluster.Name).
					Msg("Cluster not validated by LLM, but activated with LLM-generated name")
			}

			cluster.Activate()
		} else {
			// No LLM validator - activate with generated name based on ID for uniqueness
			cluster.Name = fmt.Sprintf("Cluster %s", cluster.ID.String()[:8])
			cluster.Summary = fmt.Sprintf("Cluster of %d related documents", len(community.Members))
			cluster.Activate()
		}
	}

	// Filter to only active clusters
	activeClusters := make([]*entity.DocumentCluster, 0)
	for _, cluster := range clusters {
		if cluster.Status == entity.ClusterStatusActive {
			activeClusters = append(activeClusters, cluster)
		}
	}

	// Step 5: Persist clusters (incremental update)
	// Get existing clusters to preserve names and metadata when updating
	existingClusters, _ := s.clusterRepo.GetClustersByWorkspace(ctx, workspaceID)
	existingClustersMap := make(map[entity.ClusterID]*entity.DocumentCluster)
	for _, ec := range existingClusters {
		existingClustersMap[ec.ID] = ec
	}

	for _, cluster := range activeClusters {
		// Check if cluster already exists - preserve name and summary if it does
		if existing, exists := existingClustersMap[cluster.ID]; exists {
			// Preserve existing name/summary only when membership is unchanged
			membersChanged := false
			if community, ok := communityByClusterID[cluster.ID]; ok && community != nil {
				existingMembers, err := s.clusterRepo.GetClusterMembers(ctx, workspaceID, cluster.ID)
				if err == nil {
					membersChanged = !clusterMembersMatch(existingMembers, community.Members)
				} else {
					s.logger.Warn().
						Err(err).
						Str("cluster_id", cluster.ID.String()).
						Msg("Failed to load existing cluster members for comparison")
				}
			}
			if !membersChanged {
				if existing.Name != "" {
					cluster.Name = existing.Name
				}
				if existing.Summary != "" {
					cluster.Summary = existing.Summary
				}
			}
			// Preserve created_at for existing clusters
			cluster.CreatedAt = existing.CreatedAt
			s.logger.Debug().
				Str("cluster_id", cluster.ID.String()).
				Msg("Updating existing cluster incrementally")
		}

		if err := s.clusterRepo.UpsertCluster(ctx, cluster); err != nil {
			s.logger.Warn().
				Err(err).
				Str("cluster_id", cluster.ID.String()).
				Msg("Failed to persist cluster")
			continue
		}

		// Find matching community
		community := communityByClusterID[cluster.ID]
		if community == nil && len(communities) > 0 {
			community = communities[0]
		}

		// Update memberships incrementally: clear old ones and add new ones
		// This allows clusters to evolve as documents are added/removed
		if err := s.clusterRepo.ClearClusterMemberships(ctx, workspaceID, cluster.ID); err != nil {
			s.logger.Warn().
				Err(err).
				Str("cluster_id", cluster.ID.String()).
				Msg("Failed to clear old memberships")
		}

		// Add current memberships
		for _, memberID := range community.Members {
			membership := entity.NewClusterMembership(cluster.ID, memberID, workspaceID, 1.0)
			for _, central := range cluster.CentralNodes {
				if central == memberID {
					membership.IsCentral = true
					break
				}
			}
			if err := s.clusterRepo.AddMembership(ctx, membership); err != nil {
				s.logger.Warn().
					Err(err).
					Str("cluster_id", cluster.ID.String()).
					Str("document_id", memberID.String()).
					Msg("Failed to add cluster membership")
			}
		}
	}

	// Step 5b: Mark clusters that no longer exist as disbanded
	// This happens when communities disappear (e.g., documents removed, graph changes)
	newClusterIDs := make(map[entity.ClusterID]bool)
	for _, cluster := range activeClusters {
		newClusterIDs[cluster.ID] = true
	}
	for existingID, existingCluster := range existingClustersMap {
		if !newClusterIDs[existingID] && existingCluster.Status == entity.ClusterStatusActive {
			s.logger.Debug().
				Str("cluster_id", existingID.String()).
				Msg("Cluster no longer exists in new communities, marking as disbanded")
			if err := s.clusterRepo.UpdateClusterStatus(ctx, workspaceID, existingID, entity.ClusterStatusDisbanded); err != nil {
				s.logger.Warn().
					Err(err).
					Str("cluster_id", existingID.String()).
					Msg("Failed to mark cluster as disbanded")
			}
		}
	}

	// Step 6: Auto-create projects if configured
	if s.config.CreateProjectsAuto && s.projectRepo != nil {
		projectsCreated, assignmentsMade, err := s.createProjectsFromClusters(ctx, workspaceID, activeClusters, communities)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to create projects from clusters")
		} else {
			result.ProjectsCreated = projectsCreated
			result.AssignmentsMade = assignmentsMade
		}
	}

	result.Clusters = activeClusters
	result.Duration = time.Since(start)

	s.logger.Info().
		Int("clusters", len(activeClusters)).
		Int("projects_created", result.ProjectsCreated).
		Int("assignments", result.AssignmentsMade).
		Dur("duration", result.Duration).
		Msg("Clustering pipeline complete")

	return result, nil
}

func clusterMembersMatch(existing []entity.DocumentID, proposed []entity.DocumentID) bool {
	if len(existing) != len(proposed) {
		return false
	}

	existingSet := make(map[entity.DocumentID]struct{}, len(existing))
	for _, id := range existing {
		existingSet[id] = struct{}{}
	}
	for _, id := range proposed {
		if _, ok := existingSet[id]; !ok {
			return false
		}
	}
	return true
}

// createProjectsFromClusters creates projects from validated clusters.
func (s *Service) createProjectsFromClusters(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	clusters []*entity.DocumentCluster,
	communities []*Community,
) (projectsCreated int, assignmentsMade int, err error) {
	for i, cluster := range clusters {
		if cluster.Confidence < s.config.AutoAssignThreshold {
			continue
		}

		// Check if project with this name already exists
		existingProject, err := s.findProjectByName(ctx, workspaceID, cluster.Name)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("name", cluster.Name).
				Msg("Failed to check for existing project")
			continue
		}

		var projectID entity.ProjectID
		if existingProject != nil {
			projectID = existingProject.ID
		} else {
			// Create new project
			project := entity.NewProject(workspaceID, cluster.Name, nil)
			project.Description = cluster.Summary
			project.Nature = entity.NatureGeneric // Will be refined by LLM

			if err := s.projectRepo.Create(ctx, workspaceID, project); err != nil {
				s.logger.Warn().
					Err(err).
					Str("name", cluster.Name).
					Msg("Failed to create project")
				continue
			}

			projectID = project.ID
			projectsCreated++
		}

		// Create assignments for all members using project-document relationship
		if i < len(communities) {
			for _, memberID := range communities[i].Members {
				if err := s.projectRepo.AddDocument(ctx, workspaceID, projectID, memberID, entity.ProjectDocumentRolePrimary); err != nil {
					s.logger.Debug().
						Err(err).
						Str("project_id", projectID.String()).
						Str("document_id", memberID.String()).
						Msg("Failed to add document to project")
					continue
				}
				assignmentsMade++
			}
		}
	}

	return projectsCreated, assignmentsMade, nil
}

// findProjectByName finds a project by name.
func (s *Service) findProjectByName(ctx context.Context, workspaceID entity.WorkspaceID, name string) (*entity.Project, error) {
	// Use GetByName if available, otherwise fall back to List
	project, err := s.projectRepo.GetByName(ctx, workspaceID, name, nil)
	if err != nil {
		return nil, err
	}
	return project, nil
}

// GetClusters retrieves all active clusters for a workspace.
func (s *Service) GetClusters(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentCluster, error) {
	return s.clusterRepo.GetActiveClustersByWorkspace(ctx, workspaceID)
}

// GetClusterMembers retrieves all members of a cluster.
func (s *Service) GetClusterMembers(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) ([]entity.DocumentID, error) {
	return s.clusterRepo.GetClusterMembers(ctx, workspaceID, clusterID)
}

// GetClusterMembersWithInfo retrieves all cluster members with file information.
func (s *Service) GetClusterMembersWithInfo(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) ([]*repository.ClusterMemberInfo, error) {
	return s.clusterRepo.GetClusterMembersWithInfo(ctx, workspaceID, clusterID)
}

// GetDocumentClusters retrieves all clusters a document belongs to.
func (s *Service) GetDocumentClusters(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) ([]*entity.DocumentCluster, error) {
	memberships, err := s.clusterRepo.GetMembershipsByDocument(ctx, workspaceID, documentID)
	if err != nil {
		return nil, err
	}

	clusters := make([]*entity.DocumentCluster, 0, len(memberships))
	for _, membership := range memberships {
		cluster, err := s.clusterRepo.GetCluster(ctx, workspaceID, membership.ClusterID)
		if err != nil {
			continue
		}
		if cluster != nil && cluster.Status == entity.ClusterStatusActive {
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

// MergeClusters merges two clusters into one.
func (s *Service) MergeClusters(ctx context.Context, workspaceID entity.WorkspaceID, targetID, sourceID entity.ClusterID) error {
	target, err := s.clusterRepo.GetCluster(ctx, workspaceID, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("target cluster not found")
	}

	source, err := s.clusterRepo.GetCluster(ctx, workspaceID, sourceID)
	if err != nil {
		return err
	}
	if source == nil {
		return fmt.Errorf("source cluster not found")
	}

	// Move all memberships from source to target
	sourceMembers, err := s.clusterRepo.GetMembershipsByCluster(ctx, workspaceID, sourceID)
	if err != nil {
		return err
	}

	for _, membership := range sourceMembers {
		newMembership := entity.NewClusterMembership(targetID, membership.DocumentID, workspaceID, membership.Score)
		if err := s.clusterRepo.AddMembership(ctx, newMembership); err != nil {
			s.logger.Warn().
				Err(err).
				Str("document_id", membership.DocumentID.String()).
				Msg("Failed to move membership")
		}
	}

	// Update target cluster count
	target.MemberCount += source.MemberCount
	target.UpdatedAt = time.Now()
	if err := s.clusterRepo.UpsertCluster(ctx, target); err != nil {
		return err
	}

	// Mark source as merged
	source.MergeInto(targetID)
	return s.clusterRepo.UpsertCluster(ctx, source)
}

// DisbandCluster disbands a cluster.
func (s *Service) DisbandCluster(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) error {
	cluster, err := s.clusterRepo.GetCluster(ctx, workspaceID, clusterID)
	if err != nil {
		return err
	}
	if cluster == nil {
		return fmt.Errorf("cluster not found")
	}

	cluster.Disband()
	return s.clusterRepo.UpsertCluster(ctx, cluster)
}

// GetClusterStats retrieves clustering statistics for a workspace.
func (s *Service) GetClusterStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.ClusterStats, error) {
	return s.clusterRepo.GetClusterStats(ctx, workspaceID)
}

// LoadGraph loads the persisted document graph for a workspace.
func (s *Service) LoadGraph(ctx context.Context, workspaceID entity.WorkspaceID, minEdgeWeight float64) (*entity.DocumentGraph, error) {
	return s.clusterRepo.LoadGraph(ctx, workspaceID, minEdgeWeight)
}
