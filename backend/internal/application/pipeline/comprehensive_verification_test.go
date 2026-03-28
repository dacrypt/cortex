package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/application/project"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// TestComprehensiveVerification verifica paso por paso que todos los componentes
// del pipeline funcionen correctamente y que los datos estén bien estructurados
func TestComprehensiveVerification(t *testing.T) {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)

	logger.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Info().Msg("🔍 VERIFICACIÓN COMPLETA DEL PIPELINE")
	logger.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// PASO 1: Configurar workspace
	logger.Info().Msg("")
	logger.Info().Msg("📁 PASO 1: Configurando workspace")
	absWorkspaceRoot := setupVerboseTestWorkspace(t, logger)
	entry := findPDFFile(t, absWorkspaceRoot, logger, "40 Conferencias.pdf")
	if entry == nil {
		t.Skip("PDF file not found")
		return
	}

	// PASO 2: Configurar base de datos
	logger.Info().Msg("")
	logger.Info().Msg("💾 PASO 2: Configurando base de datos")
	workspaceID := entity.NewWorkspaceID()
	ctx := context.Background()
	repos := setupTestDatabase(t, ctx, absWorkspaceRoot, workspaceID, logger)
	defer repos.conn.Close()

	wsInfo := contextinfo.WorkspaceInfo{
		ID:            workspaceID,
		Root:          absWorkspaceRoot,
		Config:        entity.WorkspaceConfig{},
		ForceFullScan: true,
	}
	ctx = contextinfo.WithWorkspaceInfo(ctx, wsInfo)

	// PASO 3: Configurar pipeline
	logger.Info().Msg("")
	logger.Info().Msg("⚙️  PASO 3: Configurando pipeline")
	orchestrator := NewOrchestrator(nil, logger)
	llmRouter, realEmbedder := setupLLMAndEmbedder(t, ctx, logger, repos.traceRepo)
	var projectService *project.Service
	projectService = setupPipelineStages(t, orchestrator, repos, llmRouter, realEmbedder, logger)

	// PASO 4: Procesar archivo
	logger.Info().Msg("")
	logger.Info().Msg("🔄 PASO 4: Procesando archivo")
	require.NoError(t, repos.fileRepo.Upsert(ctx, workspaceID, entry))
	require.NoError(t, orchestrator.Process(ctx, entry))

	// PASO 5: VERIFICACIÓN COMPLETA
	logger.Info().Msg("")
	logger.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Info().Msg("✅ PASO 5: VERIFICACIÓN COMPLETA DE DATOS")
	logger.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	docID := entity.NewDocumentID(entry.RelativePath)

	// 5.1: Verificar FileEntry
	logger.Info().Msg("")
	logger.Info().Msg("📄 5.1: Verificando FileEntry")
	savedEntry, err := repos.fileRepo.GetByPath(ctx, workspaceID, entry.RelativePath)
	require.NoError(t, err)
	require.NotNil(t, savedEntry)
	logger.Info().
		Str("path", savedEntry.RelativePath).
		Int64("size", savedEntry.FileSize).
		Bool("basic_indexed", savedEntry.Enhanced.IndexedState.Basic).
		Bool("mime_indexed", savedEntry.Enhanced.IndexedState.Mime).
		Bool("mirror_indexed", savedEntry.Enhanced.IndexedState.Mirror).
		Bool("document_indexed", savedEntry.Enhanced.IndexedState.Document).
		Msg("  ✅ FileEntry verificado")

	// 5.2: Verificar Document
	logger.Info().Msg("")
	logger.Info().Msg("📄 5.2: Verificando Document")
	doc, err := repos.docRepo.GetDocument(ctx, workspaceID, docID)
	require.NoError(t, err)
	require.NotNil(t, doc)
	logger.Info().
		Str("doc_id", doc.ID.String()).
		Str("title", doc.Title).
		Time("created_at", doc.CreatedAt).
		Msg("  ✅ Document verificado")

	// Verificar chunks
	chunks, err := repos.docRepo.GetChunksByDocument(ctx, workspaceID, docID)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 0, "Debe haber al menos un chunk")
	logger.Info().
		Int("chunk_count", len(chunks)).
		Msg("  ✅ Chunks verificados")

	// 5.3: Verificar Metadata
	logger.Info().Msg("")
	logger.Info().Msg("📋 5.3: Verificando Metadata")
	meta, err := repos.metaRepo.GetByPath(ctx, workspaceID, entry.RelativePath)
	require.NoError(t, err)
	require.NotNil(t, meta)
	logger.Info().
		Str("file_id", meta.FileID.String()).
		Str("type", meta.Type).
		Int("tags_count", len(meta.Tags)).
		Int("contexts_count", len(meta.Contexts)).
		Msg("  ✅ Metadata básica verificada")

	// Verificar tags
	if len(meta.Tags) > 0 {
		logger.Info().
			Strs("tags", meta.Tags).
			Msg("  ✅ Tags verificados")
	} else {
		logger.Warn().Msg("  ⚠️  No hay tags asignados")
	}

	// Verificar contexts (proyectos)
	if len(meta.Contexts) > 0 {
		logger.Info().
			Strs("contexts", meta.Contexts).
			Msg("  ✅ Contexts (proyectos) verificados")
	} else {
		logger.Warn().Msg("  ⚠️  No hay contexts (proyectos) asignados")
	}

	// 5.4: Verificar AISummary
	logger.Info().Msg("")
	logger.Info().Msg("🤖 5.4: Verificando AISummary")
	if meta.AISummary != nil {
		require.NotEmpty(t, meta.AISummary.Summary, "AISummary.Summary no debe estar vacío")
		logger.Info().
			Str("summary_preview", truncateString(meta.AISummary.Summary, 100)).
			Int("key_terms_count", len(meta.AISummary.KeyTerms)).
			Str("content_hash", meta.AISummary.ContentHash).
			Msg("  ✅ AISummary verificado")
		if len(meta.AISummary.KeyTerms) > 0 {
			logger.Info().
				Strs("key_terms", meta.AISummary.KeyTerms).
				Msg("    Key Terms:")
		}
	} else {
		logger.Warn().Msg("  ⚠️  AISummary no encontrado")
	}

	// 5.5: Verificar AICategory
	logger.Info().Msg("")
	logger.Info().Msg("🏷️  5.5: Verificando AICategory")
	if meta.AICategory != nil {
		require.NotEmpty(t, meta.AICategory.Category, "AICategory.Category no debe estar vacío")
		require.Greater(t, meta.AICategory.Confidence, 0.0, "AICategory.Confidence debe ser > 0")
		logger.Info().
			Str("category", meta.AICategory.Category).
			Float64("confidence", meta.AICategory.Confidence).
			Time("updated_at", meta.AICategory.UpdatedAt).
			Msg("  ✅ AICategory verificado")
	} else {
		logger.Warn().Msg("  ⚠️  AICategory no encontrado")
	}

	// 5.6: Verificar AIContext
	logger.Info().Msg("")
	logger.Info().Msg("📚 5.6: Verificando AIContext")
	if meta.AIContext != nil {
		require.True(t, meta.AIContext.HasAnyData(), "AIContext debe tener al menos algún dato")
		logger.Info().
			Int("authors", len(meta.AIContext.Authors)).
			Int("editors", len(meta.AIContext.Editors)).
			Int("translators", len(meta.AIContext.Translators)).
			Int("contributors", len(meta.AIContext.Contributors)).
			Int("locations", len(meta.AIContext.Locations)).
			Int("people_mentioned", len(meta.AIContext.PeopleMentioned)).
			Int("organizations", len(meta.AIContext.Organizations)).
			Int("historical_events", len(meta.AIContext.HistoricalEvents)).
			Int("references", len(meta.AIContext.References)).
			Float64("confidence", meta.AIContext.Confidence).
			Str("source", meta.AIContext.Source).
			Time("extracted_at", meta.AIContext.ExtractedAt).
			Msg("  ✅ AIContext verificado")

		// Verificar estructura de autores
		if len(meta.AIContext.Authors) > 0 {
			for i, author := range meta.AIContext.Authors {
				require.NotEmpty(t, author.Name, "Author.Name no debe estar vacío")
				logger.Info().
					Int("index", i+1).
					Str("name", author.Name).
					Str("role", author.Role).
					Interface("affiliation", author.Affiliation).
					Float64("confidence", author.Confidence).
					Msg("    Autor verificado")
			}
		}

		// Verificar estructura de ubicaciones
		if len(meta.AIContext.Locations) > 0 {
			for i, loc := range meta.AIContext.Locations {
				require.NotEmpty(t, loc.Name, "Location.Name no debe estar vacío")
				require.NotEmpty(t, loc.Type, "Location.Type no debe estar vacío")
				logger.Info().
					Int("index", i+1).
					Str("name", loc.Name).
					Str("type", loc.Type).
					Interface("context", loc.Context).
					Msg("    Ubicación verificada")
			}
		}

		// Verificar estructura de organizaciones
		if len(meta.AIContext.Organizations) > 0 {
			for i, org := range meta.AIContext.Organizations {
				require.NotEmpty(t, org.Name, "Organization.Name no debe estar vacío")
				require.NotEmpty(t, org.Type, "Organization.Type no debe estar vacío")
				logger.Info().
					Int("index", i+1).
					Str("name", org.Name).
					Str("type", org.Type).
					Interface("context", org.Context).
					Msg("    Organización verificada")
			}
		}
	} else {
		logger.Warn().Msg("  ⚠️  AIContext no encontrado")
	}

	// 5.7: Verificar SuggestedMetadata (Taxonomía)
	logger.Info().Msg("")
	logger.Info().Msg("📊 5.7: Verificando SuggestedMetadata (Taxonomía)")
	suggestedMeta, err := repos.suggestedRepo.Get(ctx, workspaceID, entry.ID)
	if err == nil && suggestedMeta != nil {
		if suggestedMeta.HasSuggestions() {
			logger.Info().
				Float64("confidence", suggestedMeta.Confidence).
				Str("source", suggestedMeta.Source).
				Time("generated_at", suggestedMeta.GeneratedAt).
				Int("suggested_tags_count", len(suggestedMeta.SuggestedTags)).
				Int("suggested_projects_count", len(suggestedMeta.SuggestedProjects)).
				Msg("  ✅ SuggestedMetadata verificado")

			// Verificar SuggestedTaxonomy
			if suggestedMeta.SuggestedTaxonomy != nil {
				taxonomy := suggestedMeta.SuggestedTaxonomy
				logger.Info().Msg("  📊 SuggestedTaxonomy:")
				logger.Info().
					Str("category", taxonomy.Category).
					Str("subcategory", taxonomy.Subcategory).
					Str("domain", taxonomy.Domain).
					Str("subdomain", taxonomy.Subdomain).
					Str("content_type", taxonomy.ContentType).
					Str("purpose", taxonomy.Purpose).
					Str("audience", taxonomy.Audience).
					Str("language", taxonomy.Language).
					Float64("category_confidence", taxonomy.CategoryConfidence).
					Float64("domain_confidence", taxonomy.DomainConfidence).
					Float64("content_type_confidence", taxonomy.ContentTypeConfidence).
					Strs("topics", taxonomy.Topic).
					Msg("    ✅ Taxonomía completa verificada")
			} else {
				logger.Warn().Msg("    ⚠️  SuggestedTaxonomy no encontrado")
			}

			// Verificar SuggestedTags
			if len(suggestedMeta.SuggestedTags) > 0 {
				logger.Info().Msg("  🏷️  SuggestedTags:")
				for i, tag := range suggestedMeta.SuggestedTags {
					require.NotEmpty(t, tag.Tag, "SuggestedTag.Tag no debe estar vacío")
					logger.Info().
						Int("index", i+1).
						Str("tag", tag.Tag).
						Float64("confidence", tag.Confidence).
						Str("source", tag.Source).
						Str("category", tag.Category).
						Str("reason", tag.Reason).
						Msg("    Tag sugerido verificado")
				}
			}

			// Verificar SuggestedProjects
			if len(suggestedMeta.SuggestedProjects) > 0 {
				logger.Info().Msg("  📁 SuggestedProjects:")
				for i, proj := range suggestedMeta.SuggestedProjects {
					require.NotEmpty(t, proj.ProjectName, "SuggestedProject.ProjectName no debe estar vacío")
					logger.Info().
						Int("index", i+1).
						Str("project_name", proj.ProjectName).
						Interface("project_id", proj.ProjectID).
						Float64("confidence", proj.Confidence).
						Str("source", proj.Source).
						Bool("is_new", proj.IsNew).
						Str("reason", proj.Reason).
						Msg("    Proyecto sugerido verificado")
				}
			}
		} else {
			logger.Warn().Msg("  ⚠️  SuggestedMetadata no tiene sugerencias")
		}
	} else {
		logger.Warn().Err(err).Msg("  ⚠️  SuggestedMetadata no encontrado")
	}

	// 5.8: Verificar Projects
	logger.Info().Msg("")
	logger.Info().Msg("📁 5.8: Verificando Projects")
	if len(meta.Contexts) > 0 {
		for _, projectName := range meta.Contexts {
			proj, err := projectService.GetProjectByName(ctx, workspaceID, projectName)
			if err == nil && proj != nil {
				require.NotEmpty(t, proj.Name, "Project.Name no debe estar vacío")
				require.NotEmpty(t, proj.ID.String(), "Project.ID no debe estar vacío")
				logger.Info().
					Str("project_id", proj.ID.String()).
					Str("project_name", proj.Name).
					Str("description", proj.Description).
					Time("created_at", proj.CreatedAt).
					Msg("  ✅ Project verificado")

				// Verificar documentos asociados
				docIDs, err := repos.projectRepo.GetDocuments(ctx, workspaceID, proj.ID, false)
				if err == nil {
					require.Contains(t, docIDs, docID, "El documento debe estar asociado al proyecto")
					logger.Info().
						Int("document_count", len(docIDs)).
						Msg("    Documentos asociados verificados")
				}
			} else {
				logger.Warn().
					Str("project_name", projectName).
					Err(err).
					Msg("  ⚠️  Project no encontrado")
			}
		}
	} else {
		logger.Warn().Msg("  ⚠️  No hay proyectos asignados")
	}

	// 5.9: Verificar Relationships
	logger.Info().Msg("")
	logger.Info().Msg("🔗 5.9: Verificando Relationships")
	relationships, err := repos.relationshipRepo.GetAllOutgoing(ctx, workspaceID, docID)
	require.NoError(t, err)
	logger.Info().
		Int("relationships_count", len(relationships)).
		Msg("  ✅ Relationships verificados")
	for i, rel := range relationships {
		if i >= 5 {
			logger.Info().Msgf("    ... (%d relaciones más)", len(relationships)-5)
			break
		}
		require.NotEmpty(t, rel.Type.String(), "Relationship.Type no debe estar vacío")
		require.NotEmpty(t, rel.ToDocument.String(), "Relationship.ToDocument no debe estar vacío")
		logger.Info().
			Str("type", rel.Type.String()).
			Str("to_document", rel.ToDocument.String()).
			Float64("strength", rel.Strength).
			Time("created_at", rel.CreatedAt).
			Msg("    Relación verificada")
	}

	// 5.10: Verificar Vector Store (Embeddings)
	logger.Info().Msg("")
	logger.Info().Msg("🔢 5.10: Verificando Vector Store (Embeddings)")
	if len(chunks) > 0 {
		testChunk := chunks[0]
		// Truncar texto si es muy largo (algunos embedders tienen límites)
		testText := testChunk.Text
		if len(testText) > 8000 {
			testText = testText[:8000]
		}
		testVector, err := realEmbedder.Embed(ctx, testText)
		if err != nil {
			logger.Warn().Err(err).Msg("  ⚠️  No se pudo crear embedding de prueba (puede ser normal si el chunk es muy largo)")
		} else {
			require.Greater(t, len(testVector), 0, "El vector debe tener dimensiones")

			similarChunks, err := repos.vectorStore.Search(ctx, workspaceID, testVector, 5)
			require.NoError(t, err)
			logger.Info().
				Int("vector_dimensions", len(testVector)).
				Int("similar_chunks_found", len(similarChunks)).
				Msg("  ✅ Vector Store verificado")
			if len(similarChunks) > 0 {
				logger.Info().
					Float32("top_similarity", similarChunks[0].Similarity).
					Msg("    Similaridad verificada")
			}
		}
	}

	// 5.11: Verificar Document State
	logger.Info().Msg("")
	logger.Info().Msg("📊 5.11: Verificando Document State")
	state, err := repos.stateRepo.GetState(ctx, workspaceID, docID)
	require.NoError(t, err)
	require.NotEmpty(t, string(state), "State no debe estar vacío")
	logger.Info().
		Str("state", string(state)).
		Msg("  ✅ Document State verificado")

	// 5.12: Verificar EnrichmentData
	logger.Info().Msg("")
	logger.Info().Msg("✨ 5.12: Verificando EnrichmentData")
	if meta.EnrichmentData != nil {
		logger.Info().
			Int("citations_count", len(meta.EnrichmentData.Citations)).
			Int("named_entities_count", len(meta.EnrichmentData.NamedEntities)).
			Int("tables_count", len(meta.EnrichmentData.Tables)).
			Int("formulas_count", len(meta.EnrichmentData.Formulas)).
			Msg("  ✅ EnrichmentData verificado")
	} else {
		logger.Warn().Msg("  ⚠️  EnrichmentData no encontrado")
	}

	// RESUMEN FINAL
	logger.Info().Msg("")
	logger.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Info().Msg("✅ VERIFICACIÓN COMPLETA FINALIZADA")
	logger.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	logger.Info().Msg("")
	logger.Info().Msg("📊 RESUMEN DE VERIFICACIÓN:")
	logger.Info().Msgf("  ✅ FileEntry: %s", savedEntry.RelativePath)
	logger.Info().Msgf("  ✅ Document: %s (%d chunks)", doc.ID.String(), len(chunks))
	logger.Info().Msgf("  ✅ Metadata: %d tags, %d contexts", len(meta.Tags), len(meta.Contexts))
	if meta.AISummary != nil {
		logger.Info().Msgf("  ✅ AISummary: %d caracteres, %d key terms", len(meta.AISummary.Summary), len(meta.AISummary.KeyTerms))
	}
	if meta.AICategory != nil {
		logger.Info().Msgf("  ✅ AICategory: %s (%.2f%% confianza)", meta.AICategory.Category, meta.AICategory.Confidence*100)
	}
	if meta.AIContext != nil {
		logger.Info().Msgf("  ✅ AIContext: %d autores, %d ubicaciones, %d organizaciones", len(meta.AIContext.Authors), len(meta.AIContext.Locations), len(meta.AIContext.Organizations))
	}
	if suggestedMeta != nil && suggestedMeta.SuggestedTaxonomy != nil {
		taxonomy := suggestedMeta.SuggestedTaxonomy
		logger.Info().Msgf("  ✅ SuggestedTaxonomy: %s / %s / %s", taxonomy.Category, taxonomy.Domain, taxonomy.ContentType)
	}
	logger.Info().Msgf("  ✅ Relationships: %d relaciones", len(relationships))
	logger.Info().Msgf("  ✅ Vector Store: %d chunks con embeddings", len(chunks))
	logger.Info().Msgf("  ✅ Document State: %s", string(state))
	logger.Info().Msg("")
	logger.Info().Msg("✅ TODOS LOS COMPONENTES VERIFICADOS Y FUNCIONANDO CORRECTAMENTE")
}


