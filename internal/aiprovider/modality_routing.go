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

func pickModalityModel(current, configured, modality string, gateway []client.ModelInfo) (string, error) {
	current = strings.TrimSpace(current)
	if current != "" {
		if info := modelInfoByID(gateway, current); info != nil && client.ModelHasCapability(info.Capabilities, modality) {
			return info.ID, nil
		}
	}
	configured = strings.TrimSpace(configured)
	if configured == "" {
		if auto := findFirstModelWithCapability(gateway, modality); auto != "" {
			return auto, nil
		}
		return "", fmt.Errorf("未配置%s模型：请在 AI 提供商设置中为「%s」选择默认模型", modalityLabel(modality), modalityLabel(modality))
	}
	resolved := resolveConfiguredModelID(configured, gateway)
	if info := modelInfoByID(gateway, resolved); info != nil && !client.ModelHasCapability(info.Capabilities, modality) {
		return "", fmt.Errorf("提供商配置的 %s 模型「%s」在上游列表中未标注 %s 能力，请重新选择", modalityLabel(modality), resolved, modalityLabel(modality))
	}
	return resolved, nil
}

func modalityLabel(modality string) string {
	switch modality {
	case client.ModalityImage:
		return "图片"
	case client.ModalityAudio:
		return "音频"
	case client.ModalityVideo:
		return "视频"
	default:
		return modality
	}
}

func providerSupportsImageInput(provider *providerRow) error {
	if provider == nil {
		return fmt.Errorf("AI 提供商未配置")
	}
	mm := providerModalityModels(provider)
	if strings.TrimSpace(mm.Image) != "" {
		return nil
	}
	return fmt.Errorf("当前 AI 提供商未配置图片模型：请在「AI 提供商」设置中指定图片默认模型")
}

func providerAllowsImageUpload(provider *providerRow) bool {
	return providerSupportsImageInput(provider) == nil
}

func modelSupportsModality(providerType, model, modality string, gateway []client.ModelInfo) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	if info := modelInfoByID(gateway, model); info != nil {
		return client.ModelHasCapability(info.Capabilities, modality)
	}
	return client.ModelHasCapability(client.InferModelCapabilities(model, nil), modality)
}
