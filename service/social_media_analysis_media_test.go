package service

import (
	"context"
	"testing"
)

type recordingMediaAnalyzer struct {
	imageCalls int
	videoCalls int
}

func (analyzer *recordingMediaAnalyzer) AnalyzeImage(context.Context, ImageAnalysisInput) (MediaAnalysisResult, error) {
	analyzer.imageCalls++
	return MediaAnalysisResult{Facts: []string{"image fact"}, Confidence: 0.8}, nil
}

func (analyzer *recordingMediaAnalyzer) AnalyzeVideo(context.Context, PreparedVideo) (MediaAnalysisResult, error) {
	analyzer.videoCalls++
	return MediaAnalysisResult{Facts: []string{"video fact"}, Confidence: 0.7}, nil
}

type recordingVideoPreprocessor struct {
	calls int
}

func (processor *recordingVideoPreprocessor) Prepare(context.Context, VideoInput) (PreparedVideo, error) {
	processor.calls++
	return PreparedVideo{FrameURLs: []string{"frame-1.jpg"}, Transcript: "spoken words"}, nil
}

func TestAnalyzeMediaRoutesImageToImageAnalyzer(t *testing.T) {
	analyzer := &recordingMediaAnalyzer{}
	processor := &recordingVideoPreprocessor{}

	result, err := AnalyzeMedia(context.Background(), MediaReference{Type: MediaTypeImage, URL: "https://example.com/image.jpg"}, analyzer, processor)
	if err != nil {
		t.Fatalf("analyze image: %v", err)
	}
	if analyzer.imageCalls != 1 || analyzer.videoCalls != 0 {
		t.Fatalf("unexpected analyzer calls: image=%d video=%d", analyzer.imageCalls, analyzer.videoCalls)
	}
	if processor.calls != 0 {
		t.Fatalf("image analysis should not preprocess video: %d calls", processor.calls)
	}
	if len(result.Facts) != 1 || result.Facts[0] != "image fact" {
		t.Fatalf("unexpected image result: %#v", result)
	}
}

func TestAnalyzeMediaPreprocessesVideoBeforeVideoAnalyzer(t *testing.T) {
	analyzer := &recordingMediaAnalyzer{}
	processor := &recordingVideoPreprocessor{}

	result, err := AnalyzeMedia(context.Background(), MediaReference{Type: MediaTypeVideo, URL: "https://example.com/video.mp4"}, analyzer, processor)
	if err != nil {
		t.Fatalf("analyze video: %v", err)
	}
	if processor.calls != 1 || analyzer.videoCalls != 1 || analyzer.imageCalls != 0 {
		t.Fatalf("unexpected video pipeline calls: preprocess=%d image=%d video=%d", processor.calls, analyzer.imageCalls, analyzer.videoCalls)
	}
	if result.Confidence != 0.7 {
		t.Fatalf("unexpected video result: %#v", result)
	}
}

func TestAnalyzeMediaReturnsEmptyResultWithoutMedia(t *testing.T) {
	analyzer := &recordingMediaAnalyzer{}
	processor := &recordingVideoPreprocessor{}

	result, err := AnalyzeMedia(context.Background(), MediaReference{}, analyzer, processor)
	if err != nil {
		t.Fatalf("analyze empty media: %v", err)
	}
	if analyzer.imageCalls != 0 || analyzer.videoCalls != 0 || processor.calls != 0 {
		t.Fatalf("empty media should not call providers")
	}
	if len(result.Facts) != 0 || result.Confidence != 0 {
		t.Fatalf("expected empty media result, got %#v", result)
	}
}

func TestValidateMediaAnalysisResultRejectsInvalidConfidence(t *testing.T) {
	if err := ValidateMediaAnalysisResult(MediaAnalysisResult{Facts: []string{"fact"}, Confidence: 1.1}); err == nil {
		t.Fatal("expected confidence validation error")
	}
	if err := ValidateMediaAnalysisResult(MediaAnalysisResult{Facts: []string{"fact"}, Confidence: 0.9}); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
