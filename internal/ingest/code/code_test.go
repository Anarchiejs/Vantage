package code

import (
	"context"
	"reflect"
	"testing"

	"github.com/Anarchiejs/Observatoire/internal/model"
)

func TestMesurerFixtureAlpha(t *testing.T) {
	depot := "../../../testdata/depots/alpha"
	first, err := Mesurer(context.Background(), depot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Mesurer(context.Background(), depot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("résultats instables: %#v != %#v", first, second)
	}
	byLanguage := make(map[string]model.MesureLangage)
	for _, measurement := range first {
		byLanguage[measurement.Langage] = measurement
	}
	for _, language := range []string{"Go", "TypeScript", "SQL", "Markdown"} {
		measurement, ok := byLanguage[language]
		if !ok {
			t.Errorf("langage absent: %s", language)
			continue
		}
		if measurement.LignesCode <= 0 {
			t.Errorf("aucune ligne de code pour %s: %#v", language, measurement)
		}
	}
	if _, exists := byLanguage["Binary"]; exists {
		t.Error("le binaire ignoré a été compté")
	}
}
