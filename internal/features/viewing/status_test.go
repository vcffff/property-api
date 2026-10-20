package viewing

import (
	"encoding/json"
	"slices"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestViewingTransitionGraph(t *testing.T) {
	statuses := []ViewingStatus{ViewingStatusPending, ViewingStatusConfirmed, ViewingStatusCancelled, ViewingStatusCompleted}
	for _, from := range statuses {
		for _, to := range statuses {
			want := from == ViewingStatusPending && (to == ViewingStatusConfirmed || to == ViewingStatusCancelled) ||
				from == ViewingStatusConfirmed && (to == ViewingStatusCompleted || to == ViewingStatusCancelled)
			if got := slices.Contains(transitionSources(to), from); got != want {
				t.Errorf("%s -> %s = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestViewingStatusIsValid(t *testing.T) {
	for _, status := range []ViewingStatus{
		ViewingStatusPending, ViewingStatusConfirmed, ViewingStatusCancelled, ViewingStatusCompleted,
	} {
		if !status.IsValid() {
			t.Errorf("status %q must be valid", status)
		}
	}
	for _, status := range []ViewingStatus{"", "approved", "Confirmed", "unknown"} {
		if status.IsValid() {
			t.Errorf("status %q must be invalid", status)
		}
	}
}

func TestViewingStatusJSONCompatibility(t *testing.T) {
	for _, status := range []ViewingStatus{
		ViewingStatusPending, ViewingStatusConfirmed, ViewingStatusCancelled, ViewingStatusCompleted,
	} {
		t.Run(string(status), func(t *testing.T) {
			data, err := json.Marshal(Viewing{ID: 7, Status: status})
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if got, want := string(fields["status"]), `"`+string(status)+`"`; got != want {
				t.Fatalf("JSON status = %s, want %s", got, want)
			}
			var decoded Viewing
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.ID != 7 || decoded.Status != status {
				t.Fatalf("unexpected round trip: %+v", decoded)
			}
		})
	}
}

func TestViewingStatusGORMCompatibility(t *testing.T) {
	model, err := schema.Parse(&Viewing{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	field := model.LookUpField("Status")
	if field == nil {
		t.Fatal("status field missing")
	}
	if field.DBName != "status" || field.DataType != "text" || field.GORMDataType != schema.String || !field.NotNull {
		t.Fatalf("unexpected status column: %+v", field)
	}
	if !field.HasDefaultValue || field.DefaultValue != "pending" || field.DefaultValueInterface != "pending" {
		t.Fatalf("unexpected status default: %+v", field)
	}
}
