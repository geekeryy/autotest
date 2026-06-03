package spec

import (
	"context"
	"fmt"

	testcase "autotest/internal/case"
	"autotest/internal/generator"

	"github.com/google/uuid"
)

type Service struct {
	repo      *Repository
	cases     *testcase.Repository
	importer  *Importer
	generator *generator.Generator
}

func NewService(repo *Repository, cases *testcase.Repository, importer *Importer, generator *generator.Generator) *Service {
	return &Service{repo: repo, cases: cases, importer: importer, generator: generator}
}

func (s *Service) Import(ctx context.Context, projectID, serviceID uuid.UUID, data []byte, syncMode SyncMode) (*ImportSummary, error) {
	result, err := s.importer.Import(ctx, data)
	if err != nil {
		return nil, err
	}

	apiSpec, err := s.repo.CreateSpec(ctx, projectID, serviceID, result, data)
	if err != nil {
		return nil, err
	}

	endpoints, createdEP, updatedEP, err := s.repo.SyncEndpoints(ctx, projectID, serviceID, apiSpec.ID, result.Endpoints)
	if err != nil {
		return nil, err
	}

	deletedEndpoints := 0
	if syncMode == SyncModeOverwrite {
		prunedIDs, err := s.repo.PruneEndpointsNotIn(ctx, serviceID, result.Endpoints)
		if err != nil {
			return nil, err
		}
		deletedEndpoints = len(prunedIDs)
		if len(prunedIDs) > 0 {
			if _, err := s.cases.SoftDeleteAutoCasesForEndpoints(ctx, prunedIDs); err != nil {
				return nil, fmt.Errorf("soft delete auto cases: %w", err)
			}
		}
	}

	generated := 0
	for _, endpoint := range endpoints {
		drafts, err := s.generator.Generate(generator.Endpoint{
			ID:             endpoint.ID,
			ProjectID:      projectID,
			ServiceID:      serviceID,
			Method:         endpoint.Method,
			Path:           endpoint.Path,
			OperationID:    endpoint.OperationID,
			Summary:        endpoint.Summary,
			RequestSchema:  endpoint.RequestSchema,
			ResponseSchema: endpoint.ResponseSchema,
		})
		if err != nil {
			return nil, fmt.Errorf("generate cases for %s %s: %w", endpoint.Method, endpoint.Path, err)
		}
		for _, draft := range drafts {
			if _, err := s.cases.UpsertGenerated(ctx, draft); err != nil {
				return nil, fmt.Errorf("upsert generated case: %w", err)
			}
			generated++
		}
	}

	if err := s.repo.PruneSpecs(ctx, serviceID, 5); err != nil {
		return nil, err
	}

	return &ImportSummary{
		SpecID:           apiSpec.ID,
		SpecVersion:      apiSpec.Version,
		ContentHash:      apiSpec.ContentHash,
		ApiCount:         len(result.Endpoints),
		CreatedEndpoints: createdEP,
		UpdatedEndpoints: updatedEP,
		GeneratedCases:   generated,
		DeletedEndpoints: deletedEndpoints,
		SyncMode:         string(syncMode),
	}, nil
}

func (s *Service) ListSpecs(ctx context.Context, projectID, serviceID uuid.UUID) ([]APISpec, error) {
	return s.repo.ListSpecs(ctx, projectID, serviceID)
}

func (s *Service) ListEndpoints(ctx context.Context, projectID, serviceID uuid.UUID) ([]Endpoint, error) {
	return s.repo.ListEndpoints(ctx, projectID, serviceID)
}
