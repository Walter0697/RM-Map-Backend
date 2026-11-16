package service

import (
	"context"
	"errors"
	"fmt"
)

var ErrUnsupportedMediaType = errors.New("unsupported media type")

type MediaAnalyzer interface {
	AnalyzeImage(context.Context, ImageAnalysisInput) (MediaAnalysisResult, error)
	AnalyzeVideo(context.Context, PreparedVideo) (MediaAnalysisResult, error)
}

type VideoPreprocessor interface {
	Prepare(context.Context, VideoInput) (PreparedVideo, error)
}

func AnalyzeMedia(ctx context.Context, media MediaReference, analyzer MediaAnalyzer, preprocessor VideoPreprocessor) (MediaAnalysisResult, error) {
	if media.Type == "" {
		return MediaAnalysisResult{}, nil
	}
	if analyzer == nil {
		return MediaAnalysisResult{}, errors.New("media analyzer is required")
	}
	switch media.Type {
	case MediaTypeImage:
		return analyzer.AnalyzeImage(ctx, ImageAnalysisInput{Media: media})
	case MediaTypeVideo:
		if preprocessor == nil {
			return MediaAnalysisResult{}, errors.New("video preprocessor is required")
		}
		prepared, err := preprocessor.Prepare(ctx, VideoInput{Media: media})
		if err != nil {
			return MediaAnalysisResult{}, fmt.Errorf("prepare video: %w", err)
		}
		return analyzer.AnalyzeVideo(ctx, prepared)
	default:
		return MediaAnalysisResult{}, fmt.Errorf("%w: %s", ErrUnsupportedMediaType, media.Type)
	}
}

func ValidateMediaAnalysisResult(result MediaAnalysisResult) error {
	if result.Confidence < 0 || result.Confidence > 1 {
		return fmt.Errorf("media confidence must be between 0 and 1")
	}
	if len(result.Facts) == 0 {
		return errors.New("media analysis must contain at least one fact")
	}
	return nil
}
