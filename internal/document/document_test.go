package document

import "testing"

func TestDocumentValidationRejectsDuplicateElementIDs(t *testing.T) {
	doc := New("receipt-1", "Receipt", Size{Width: 80_000, Height: 100_000})
	doc.Elements = []Element{{ID: "title", Type: TextElement}, {ID: "title", Type: TextElement}}
	if err := doc.Validate(); err == nil {
		t.Fatal("Validate() unexpectedly accepted duplicate element ids")
	}
}

func TestNewDocumentIsValid(t *testing.T) {
	doc := New("receipt-1", "Receipt", Size{Width: 80_000, Height: 100_000})
	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
