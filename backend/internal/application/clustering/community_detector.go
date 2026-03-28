package clustering

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// CommunityDetectorConfig contains configuration for community detection.
type CommunityDetectorConfig struct {
	MinCommunitySize int     // Minimum nodes to form a community (default: 2)
	MaxIterations    int     // Maximum Louvain iterations (default: 100)
	Resolution       float64 // Resolution parameter for Louvain (default: 1.0)
	MinModularity    float64 // Minimum modularity improvement to continue (default: 0.0001)
}

// DefaultCommunityDetectorConfig returns the default configuration.
func DefaultCommunityDetectorConfig() CommunityDetectorConfig {
	return CommunityDetectorConfig{
		MinCommunitySize: 2,
		MaxIterations:    100,
		Resolution:       1.0,
		MinModularity:    0.0001,
	}
}

// CommunityDetector detects communities in a document graph using the Louvain algorithm.
type CommunityDetector struct {
	config CommunityDetectorConfig
	logger zerolog.Logger
}

// NewCommunityDetector creates a new community detector.
func NewCommunityDetector(config CommunityDetectorConfig, logger zerolog.Logger) *CommunityDetector {
	return &CommunityDetector{
		config: config,
		logger: logger.With().Str("component", "community_detector").Logger(),
	}
}

// Community represents a detected community of documents.
type Community struct {
	ID         int
	Members    []entity.DocumentID
	Modularity float64 // Community contribution to total modularity
}

// DetectCommunities runs the Louvain algorithm to detect communities.
func (d *CommunityDetector) DetectCommunities(ctx context.Context, graph *entity.DocumentGraph) ([]*Community, error) {
	if graph.NodeCount() < d.config.MinCommunitySize {
		return nil, nil
	}

	d.logger.Info().
		Int("nodes", graph.NodeCount()).
		Int("edges", graph.EdgeCount()).
		Msg("Starting community detection")

	// Initialize: each node in its own community
	nodeToComm := make(map[entity.DocumentID]int)
	commToNodes := make(map[int][]entity.DocumentID)

	for i, node := range graph.Nodes {
		nodeToComm[node] = i
		commToNodes[i] = []entity.DocumentID{node}
	}

	// Compute total weight of the graph (sum of all edge weights * 2 for undirected)
	totalWeight := graph.TotalWeight() * 2

	if totalWeight == 0 {
		d.logger.Warn().Msg("Graph has no weighted edges")
		return nil, nil
	}

	// Compute node degrees (sum of adjacent edge weights)
	nodeDegree := make(map[entity.DocumentID]float64)
	for _, edge := range graph.Edges {
		nodeDegree[edge.FromDoc] += edge.Weight
		nodeDegree[edge.ToDoc] += edge.Weight
	}

	// Louvain Phase 1: Local moving
	improved := true
	iteration := 0

	for improved && iteration < d.config.MaxIterations {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		improved = false
		iteration++

		// Try to move each node to a neighboring community
		for _, node := range graph.Nodes {
			currentComm := nodeToComm[node]
			neighbors := graph.GetNeighborsWithWeight(node)

			if len(neighbors) == 0 {
				continue
			}

			// Calculate modularity gain for each neighboring community
			bestComm := currentComm
			bestGain := 0.0

			// Get unique neighboring communities in deterministic order
			neighborComms := make(map[int]bool)
			for neighbor := range neighbors {
				neighborComms[nodeToComm[neighbor]] = true
			}
			neighborCommIDs := make([]int, 0, len(neighborComms))
			for commID := range neighborComms {
				neighborCommIDs = append(neighborCommIDs, commID)
			}
			sort.Ints(neighborCommIDs)

			for _, targetComm := range neighborCommIDs {
				if targetComm == currentComm {
					continue
				}

				gain := d.modularityGain(node, targetComm, nodeToComm, nodeDegree, graph, totalWeight)
				if gain > bestGain {
					bestGain = gain
					bestComm = targetComm
				}
			}

			// Move node if there's positive gain
			if bestGain > d.config.MinModularity && bestComm != currentComm {
				// Remove from current community
				oldNodes := commToNodes[currentComm]
				newNodes := make([]entity.DocumentID, 0, len(oldNodes)-1)
				for _, n := range oldNodes {
					if n != node {
						newNodes = append(newNodes, n)
					}
				}
				if len(newNodes) > 0 {
					commToNodes[currentComm] = newNodes
				} else {
					delete(commToNodes, currentComm)
				}

				// Add to new community
				nodeToComm[node] = bestComm
				commToNodes[bestComm] = append(commToNodes[bestComm], node)
				improved = true
			}
		}
	}

	d.logger.Debug().
		Int("iterations", iteration).
		Int("communities", len(commToNodes)).
		Msg("Louvain phase 1 complete")

	// Convert to Community structs
	communities := make([]*Community, 0, len(commToNodes))
	for commID, members := range commToNodes {
		if len(members) >= d.config.MinCommunitySize {
			comm := &Community{
				ID:         commID,
				Members:    members,
				Modularity: d.communityModularity(members, nodeToComm, nodeDegree, graph, totalWeight),
			}
			communities = append(communities, comm)
		}
	}

	// Sort by size descending
	sort.Slice(communities, func(i, j int) bool {
		return len(communities[i].Members) > len(communities[j].Members)
	})

	// Renumber communities
	for i, comm := range communities {
		comm.ID = i
	}

	d.logger.Info().
		Int("communities", len(communities)).
		Float64("modularity", d.totalModularity(communities, nodeToComm, nodeDegree, graph, totalWeight)).
		Msg("Community detection complete")

	return communities, nil
}

// modularityGain calculates the modularity gain of moving a node to a community.
func (d *CommunityDetector) modularityGain(
	node entity.DocumentID,
	targetComm int,
	nodeToComm map[entity.DocumentID]int,
	nodeDegree map[entity.DocumentID]float64,
	graph *entity.DocumentGraph,
	totalWeight float64,
) float64 {
	// Modularity gain formula:
	// ΔQ = [Σin + ki,in] / m - [(Σtot + ki) / 2m]^2 - [Σin / m - (Σtot / 2m)^2 - (ki / 2m)^2]
	// Simplified: ΔQ ≈ ki,in / m - ki * Σtot / (2m^2)

	ki := nodeDegree[node]
	if ki == 0 {
		return 0
	}

	// Sum of weights from node to target community
	kiIn := 0.0
	neighbors := graph.GetNeighborsWithWeight(node)
	for neighbor, weight := range neighbors {
		if nodeToComm[neighbor] == targetComm {
			kiIn += weight
		}
	}

	// Sum of degrees in target community
	sigmaTot := 0.0
	for n, comm := range nodeToComm {
		if comm == targetComm {
			sigmaTot += nodeDegree[n]
		}
	}

	m := totalWeight / 2
	gain := (kiIn / m) - (d.config.Resolution * ki * sigmaTot / (2 * m * m))

	return gain
}

// communityModularity calculates the modularity contribution of a community.
func (d *CommunityDetector) communityModularity(
	members []entity.DocumentID,
	nodeToComm map[entity.DocumentID]int,
	nodeDegree map[entity.DocumentID]float64,
	graph *entity.DocumentGraph,
	totalWeight float64,
) float64 {
	if len(members) == 0 {
		return 0
	}

	// Sum of internal weights
	internalWeight := 0.0
	memberSet := make(map[entity.DocumentID]bool)
	for _, m := range members {
		memberSet[m] = true
	}

	for _, edge := range graph.Edges {
		if memberSet[edge.FromDoc] && memberSet[edge.ToDoc] {
			internalWeight += edge.Weight
		}
	}

	// Sum of degrees
	sigmaTot := 0.0
	for _, m := range members {
		sigmaTot += nodeDegree[m]
	}

	m := totalWeight / 2
	if m == 0 {
		return 0
	}

	// Q_c = Σin / 2m - (Σtot / 2m)^2
	return (internalWeight / m) - math.Pow(sigmaTot/(2*m), 2)
}

// totalModularity calculates the total modularity of the partition.
func (d *CommunityDetector) totalModularity(
	communities []*Community,
	nodeToComm map[entity.DocumentID]int,
	nodeDegree map[entity.DocumentID]float64,
	graph *entity.DocumentGraph,
	totalWeight float64,
) float64 {
	total := 0.0
	for _, comm := range communities {
		total += d.communityModularity(comm.Members, nodeToComm, nodeDegree, graph, totalWeight)
	}
	return total
}

// ConvertToDocumentClusters converts detected communities to DocumentCluster entities.
// Uses deterministic cluster IDs based on central documents to enable incremental updates.
func (d *CommunityDetector) ConvertToDocumentClusters(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	communities []*Community,
) []*entity.DocumentCluster {
	clusters := make([]*entity.DocumentCluster, 0, len(communities))

	for _, comm := range communities {
		// Determine central nodes first (used for deterministic ID generation)
		maxCentral := 3
		if len(comm.Members) < maxCentral {
			maxCentral = len(comm.Members)
		}

		// Sort members to ensure deterministic selection of central nodes
		sortedMembers := make([]entity.DocumentID, len(comm.Members))
		copy(sortedMembers, comm.Members)
		sort.Slice(sortedMembers, func(i, j int) bool {
			return string(sortedMembers[i]) < string(sortedMembers[j])
		})
		centralNodes := sortedMembers[:maxCentral]

		// Generate deterministic cluster ID based on central nodes
		// This allows matching with existing clusters for incremental updates
		seed := ""
		for _, node := range centralNodes {
			seed += string(node) + ":"
		}
		clusterID := entity.NewClusterIDFromSeed(seed)

		cluster := &entity.DocumentCluster{
			ID:           clusterID,
			WorkspaceID:  workspaceID,
			Name:         "", // Will be set later
			Summary:      "",
			Status:       entity.ClusterStatusPending,
			Confidence:   comm.Modularity,
			MemberCount:  len(comm.Members),
			CentralNodes: centralNodes,
			TopEntities:  []string{},
			TopKeywords:  []string{},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// FindCentralNodes identifies the most central nodes in a community.
func (d *CommunityDetector) FindCentralNodes(graph *entity.DocumentGraph, members []entity.DocumentID, count int) []entity.DocumentID {
	if len(members) <= count {
		return members
	}

	memberSet := make(map[entity.DocumentID]bool)
	for _, m := range members {
		memberSet[m] = true
	}

	// Calculate internal degree for each member
	type nodeScore struct {
		node  entity.DocumentID
		score float64
	}
	scores := make([]nodeScore, 0, len(members))

	for _, member := range members {
		internalWeight := 0.0
		neighbors := graph.GetNeighborsWithWeight(member)
		for neighbor, weight := range neighbors {
			if memberSet[neighbor] {
				internalWeight += weight
			}
		}
		scores = append(scores, nodeScore{node: member, score: internalWeight})
	}

	// Sort by internal degree descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Return top nodes
	result := make([]entity.DocumentID, count)
	for i := 0; i < count; i++ {
		result[i] = scores[i].node
	}

	return result
}
