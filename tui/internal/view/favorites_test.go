package view

import "testing"

func TestSortFavoritesFirstMovesFavoritesToFront(t *testing.T) {
	items := []string{"a", "b", "c", "d"}
	favorite := map[string]bool{"b": true, "d": true}

	got := sortFavoritesFirst(items, func(s string) bool { return favorite[s] })

	want := []string{"b", "d", "a", "c"}
	if len(got) != len(want) {
		t.Fatalf("sortFavoritesFirst() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sortFavoritesFirst()[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestSortFavoritesFirstNoFavoritesPreservesOrder(t *testing.T) {
	items := []int{3, 1, 4, 1, 5}

	got := sortFavoritesFirst(items, func(int) bool { return false })

	for i := range items {
		if got[i] != items[i] {
			t.Errorf("sortFavoritesFirst() = %v, want unchanged %v", got, items)
			break
		}
	}
}

func TestSortFavoritesFirstAllFavoritesPreservesOrder(t *testing.T) {
	items := []int{3, 1, 4, 1, 5}

	got := sortFavoritesFirst(items, func(int) bool { return true })

	for i := range items {
		if got[i] != items[i] {
			t.Errorf("sortFavoritesFirst() = %v, want unchanged %v", got, items)
			break
		}
	}
}

func TestSortFavoritesFirstEmptyInput(t *testing.T) {
	got := sortFavoritesFirst([]string{}, func(string) bool { return true })
	if len(got) != 0 {
		t.Errorf("sortFavoritesFirst(empty) = %v, want empty", got)
	}
}
