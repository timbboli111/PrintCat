package document

import (
	"encoding/json"
	"testing"
)

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

func TestLegacyDocumentJSONRemainsValid(t *testing.T) {
	const legacy = `{"version":1,"id":"legacy","name":"Receipt","pageSize":{"width":80000,"height":100000},"elements":[{"id":"title","type":"text","bounds":{"position":{"x":1000,"y":2000},"size":{"width":40000,"height":10000}},"visible":true,"data":{"content":"Hello","fontSize":12000}}]}`
	var doc Document
	if err := json.Unmarshal([]byte(legacy), &doc); err != nil {
		t.Fatalf("unmarshal legacy document: %v", err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("legacy document validation: %v", err)
	}
}
