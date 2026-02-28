package generated

import "testing"

func TestRoroadListOperationsRemovedFromSchema(t *testing.T) {
	if parsedSchema.Query.Fields.ForName("roroadlists") != nil {
		t.Fatalf("query field roroadlists should be removed")
	}
	if parsedSchema.Query.Fields.ForName("roroadlistsbyname") != nil {
		t.Fatalf("query field roroadlistsbyname should be removed")
	}

	if parsedSchema.Mutation.Fields.ForName("createRoroadList") != nil {
		t.Fatalf("mutation field createRoroadList should be removed")
	}
	if parsedSchema.Mutation.Fields.ForName("updateRoroadList") != nil {
		t.Fatalf("mutation field updateRoroadList should be removed")
	}
	if parsedSchema.Mutation.Fields.ForName("manageMultipleRoroadList") != nil {
		t.Fatalf("mutation field manageMultipleRoroadList should be removed")
	}
}
