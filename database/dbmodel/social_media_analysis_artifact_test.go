package dbmodel

import "testing"

func TestSocialMediaAnalysisArtifactKindsAreStable(t *testing.T) {
	if SocialMediaAnalysisArtifactKindSourceJSON != "source_json" || SocialMediaAnalysisArtifactKindImage != "image" || SocialMediaAnalysisArtifactKindVideo != "video" {
		t.Fatalf("unexpected artifact kinds")
	}
	artifact := SocialMediaAnalysisArtifact{
		AnalysisJobID:  12,
		JobID:          "analysis-12",
		ObjectKey:      "social-analysis/analysis-12/source.json",
		Kind:           SocialMediaAnalysisArtifactKindSourceJSON,
		ContentType:    "application/json",
		SizeBytes:      2,
		SHA256:         "abc",
		RetentionClass: "social-analysis-source",
	}
	if artifact.AnalysisJobID != 12 || artifact.Kind != SocialMediaAnalysisArtifactKindSourceJSON {
		t.Fatalf("unexpected artifact: %+v", artifact)
	}
}
