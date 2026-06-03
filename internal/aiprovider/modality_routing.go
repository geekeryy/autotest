package aiprovider

import (
	"context"
	"fmt"
	"strings"

	"autotest/internal/aiprovider/client"
)

type sessionModalities struct {
	image bool
	audio bool
	video bool
}

func sessionNeededModalities(history []StoredMessage, turnHasImages bool) sessionModalities {
	var need sessionModalities
	if turnHasImages || historyHasImageAttachments(history) {
		need.image = true
	}
	// Audio/video routing hooks: extend when attachments support those types.
	return need
}

func (s *Service) listGatewayModels(ctx context.Context, row *providerRow) []client.ModelInfo {
	if s == nil || row == nil {
		return nil
	}
	result, err := s.listModelsForRow(ctx, row)
	if err != nil || result == nil {
		return nil
	}
	return result.Models
}

func (s *Service) applyModalityRouting(ctx context.Context, cfg *streamConfig, history []StoredMessage) error {
	if cfg == nil || cfg.Provider == nil {
		return nil
	}
	need := sessionNeededModalities(history, cfg.TurnHasImages)
	if !need.image && !need.audio && !need.video {
		return nil
	}

	gateway := s.listGatewayModels(ctx, cfg.Provider)
	modalityCfg := providerModalityModels(cfg.Provider)
	current := strings.TrimSpace(cfg.BaseOpts.Model)

	if need.image {
		if err := providerSupportsImageInput(cfg.Provider); err != nil {
			return err
		}
		model, err := pickModalityModel(current, modalityCfg.Image, client.ModalityImage, gateway)
		if err != nil {
			return err
		}
		cfg.BaseOpts.Model = model
		cfg.VisionEnabled = true
	}
	if need.audio {
		model, err := pickModalityModel(current, modalityCfg.Audio, client.ModalityAudio, gateway)
		if err != nil {
			return err
		}
		cfg.BaseOpts.Model = model
	}
	if need.video {
		model, err := pickModalityModel(current, modalityCfg.Video, client.ModalityVideo, gateway)
		if err != nil {
			return err
		}
		cfg.BaseOpts.Model = model
	}
	return nil
}

func pickModalityModel(_ string, configured, modality string, gateway []client.ModelInfo) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return resolveConfiguredModelID(configured, gateway), nil
	}
	return "", fmt.Errorf("未配置%s模型：请在「AI 能力管理 → AI 提供商」中为「%s」选择默认模型", client.ModalityLabelZH(modality), client.ModalityLabelZH(modality))
}

func providerSupportsImageInput(provider *providerRow) error {
	if provider == nil {
		return fmt.Errorf("AI 提供商未配置")
	}
	mm := providerModalityModels(provider)
	if strings.TrimSpace(mm.Image) != "" {
		return nil
	}
	return fmt.Errorf("当前 AI 提供商未配置图片模型：请在「AI 能力管理 → AI 提供商」中指定图片默认模型")
}

