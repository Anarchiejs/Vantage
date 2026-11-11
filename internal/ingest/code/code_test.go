package code

import (
	"context"
	"reflect"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/model"
)

func TestMeasureFixtureAlpha(t *testing.T) {
	repo := "../../../testdata/depots/alpha"
	first, err := Measure(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Measure(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("unstable results: %#v != %#v", first, second)
	}
	byLanguage := make(map[string]model.LanguageStat)
	for _, measurement := range first {
		byLanguage[measurement.Language] = measurement
	}
	for _, language := range []string{"Go", "TypeScript", "SQL", "Markdown"} {
		measurement, ok := byLanguage[language]
		if !ok {
			t.Errorf("missing language: %s", language)
			continue
		}
		if measurement.CodeLines <= 0 {
			t.Errorf("no lines of code for %s: %#v", language, measurement)
		}
	}
	if _, exists := byLanguage["Binary"]; exists {
		t.Error("the ignored binary was counted")
	}
}
