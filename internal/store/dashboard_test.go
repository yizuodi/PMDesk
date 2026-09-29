package store

import "testing"

func TestDashboardUpcomingVersionsUseTransferDateOrdering(t *testing.T) {
	s, _ := openTestStore(t)
	for _, in := range []VersionInput{
		{Name: "R027C10B001", StartDate: "2028-01-01", ReleaseDate: "2025-01-01", Status: "active"},
		{Name: "R027C10B002", StartDate: "2026-01-01", ReleaseDate: "2028-01-01", Status: "tested"},
		{Name: "R027C10B003", ReleaseDate: "2024-01-01", Status: "planned"},
		{Name: "R027C10B004", StartDate: "2027-01-01", Status: "released"},
	} {
		if _, err := s.CreateVersion(in); err != nil {
			t.Fatal(err)
		}
	}

	d, err := s.GetDashboard()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.UpcomingReleases) != 3 {
		t.Fatalf("upcoming count = %d, want 3", len(d.UpcomingReleases))
	}
	want := []string{"R027C10B002", "R027C10B001", "R027C10B003"}
	for i, v := range d.UpcomingReleases {
		if v.Name != want[i] {
			t.Fatalf("upcoming order = %v at index %d, want %q", d.UpcomingReleases, i, want[i])
		}
	}
}
