package store

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name string
		want ParsedVersion
	}{
		{"R027C10", ParsedVersion{Type: VTypeC, R: 27, C: 10}},
		{"R027C10B003", ParsedVersion{Type: VTypeB, R: 27, C: 10, B: 3}},
		{"R027C00SPC200B020", ParsedVersion{Type: VTypeSPC, R: 27, C: 0, Extra: 200, B: 20}},
		{"R027C00CP1022B010", ParsedVersion{Type: VTypeCP, R: 27, C: 0, Extra: 1022, B: 10}},
		{"R27C10迭代0", ParsedVersion{Type: VTypeKey, R: 27, C: 10}},
		{"r027c10b003", ParsedVersion{Type: VTypeB, R: 27, C: 10, B: 3}},
		{" legacy ", ParsedVersion{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseVersion(tt.name); got != tt.want {
				t.Fatalf("ParseVersion(%q) = %#v, want %#v", tt.name, got, tt.want)
			}
		})
	}
}

func TestVersionCRUDAndTransferDateOrdering(t *testing.T) {
	s, _ := openTestStore(t)
	created, err := s.CreateVersion(VersionInput{
		Name: "r027c10b003", Description: "说明", StartDate: "2027-03-01", ReleaseDate: "2026-01-01", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "R027C10B003" || created.VType != VTypeB || created.BNum == nil || *created.BNum != 3 {
		t.Fatalf("structured version = %#v", created)
	}

	if _, err := s.CreateVersion(VersionInput{Name: "R027C10B004", StartDate: "2026-01-01", Status: "planned"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVersion(VersionInput{Name: "R027C10B005", Status: "planned"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVersion(VersionInput{Name: "R027C10B006", StartDate: "2028-01-01", Status: "released"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVersion(VersionInput{Name: "R027C10B007", StartDate: "2025-01-01", Status: "tested"}); err != nil {
		t.Fatal(err)
	}

	versions, err := s.ListVersions()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, v := range versions {
		names = append(names, v.Name)
	}
	want := []string{"R027C10B007", "R027C10B004", "R027C10B003", "R027C10B006", "R027C10B005"}
	if len(names) != len(want) {
		t.Fatalf("version count = %d, want %d (%v)", len(names), len(want), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("version order = %v, want %v", names, want)
		}
	}

	if err := s.UpdateVersion(created.ID, VersionInput{Name: "R027C10迭代0", Description: "更新", Status: "tested"}); err != nil {
		t.Fatal(err)
	}
	updated, err := s.GetVersion(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "R027C10迭代0" || updated.Description != "更新" || updated.Status != "tested" {
		t.Fatalf("updated version = %#v", updated)
	}
	if err := s.DeleteVersion(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetVersion(created.ID); err == nil {
		t.Fatal("deleted version still exists")
	}
}
