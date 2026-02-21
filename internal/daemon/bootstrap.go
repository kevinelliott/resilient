package daemon

import (
	"log"
	"time"

	"resilient/internal/store"
)

// BootstrapDefaultData creates an initial "Survival Pack" catalog with useful knowledge bases
// if the local node's database is completely empty.
func BootstrapDefaultData(s *store.Store) error {
	catalogs, err := s.GetCatalogs()
	if err != nil {
		return err
	}

	// Only bootstrap if the survival catalog isn't specifically present.
	for _, c := range catalogs {
		if c.Name == "Survival Knowledge Base" {
			// Already seeded.
			return nil
		}
	}

	log.Println("Initializing new database with default Survival Knowledge base...")

	now := time.Now().Unix()

	// 1. Create the Root Catalog
	survivalCat := &store.Catalog{
		ID:          "bootstrap-cat-survival",
		Name:        "Survival Knowledge Base",
		Description: "Essential human knowledge backups, curated for disaster recovery and off-grid resilience.",
		RootHash:    "genesis_bootstrap",
		CreatedAt:   now,
	}

	if err := s.InsertCatalog(survivalCat); err != nil {
		return err
	}

	// 2. Create the Medical & First Aid Folder
	medicalBundle := &store.Bundle{
		ID:             "bootstrap-bundle-medical",
		CatalogID:      survivalCat.ID,
		ParentBundleID: "",
		Type:           "folder",
		Name:           "Medical & First Aid",
		Description:    "Offline field manuals, emergency triage, and pharmaceutical references.",
		CreatedAt:      now,
	}
	s.InsertBundle(medicalBundle)

	medicalFiles := []struct {
		Title string
		Path  string
		Size  int64
		URL   string
	}{
		{
			Title: "WikiMed 2026 (English)",
			Path:  "wikipedia_en_medicine_maxi_2026-01.zim",
			Size:  2100000000,
			URL:   "https://download.kiwix.org/zim/wikipedia/wikipedia_en_medicine_maxi_2026-01.zim",
		},
	}

	for i, m := range medicalFiles {
		s.InsertFile(&store.File{
			ID:          "bootstrap-file-medical-" + string(rune(i)),
			CatalogID:   survivalCat.ID,
			BundleID:    medicalBundle.ID,
			Title:       m.Title,
			Path:        m.Path,
			Size:        m.Size,
			SourceURL:   m.URL,
			ChunkHashes: "[]", // Intentionally empty. User must click "Queue Intake" to seed the hashes from the SourceURL.
		})
	}

	// 3. Create the Practical Repair & Technology Folder
	techBundle := &store.Bundle{
		ID:             "bootstrap-bundle-tech",
		CatalogID:      survivalCat.ID,
		ParentBundleID: "",
		Type:           "folder",
		Name:           "Repair & Mechanics",
		Description:    "Guides for fixing electronics, mechanical engines, and basic infrastructure.",
		CreatedAt:      now,
	}
	s.InsertBundle(techBundle)

	techFiles := []struct {
		Title string
		Path  string
		Size  int64
		URL   string
	}{
		{
			Title: "iFixit Complete Device Manuals",
			Path:  "ifixit_en_all_2025-12.zim",
			Size:  3500000000,
			URL:   "https://download.kiwix.org/zim/ifixit/ifixit_en_all_2025-12.zim",
		},
	}

	for i, t := range techFiles {
		s.InsertFile(&store.File{
			ID:          "bootstrap-file-tech-" + string(rune(i)),
			CatalogID:   survivalCat.ID,
			BundleID:    techBundle.ID,
			Title:       t.Title,
			Path:        t.Path,
			Size:        t.Size,
			SourceURL:   t.URL,
			ChunkHashes: "[]",
		})
	}

	log.Println("Successfully bootstrapped default Survival catalog.")
	return nil
}
