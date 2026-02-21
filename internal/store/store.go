package store

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Apply concurrency and WAL pragmas explicitly
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=OFF;"); err != nil {
		return nil, fmt.Errorf("failed to set pragmas: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	// Basic schemas for the data model concepts
	queries := []string{
		`CREATE TABLE IF NOT EXISTS peers (
			id TEXT PRIMARY KEY,
			multiaddr TEXT,
			last_seen INTEGER,
			trust_level INTEGER DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS catalogs (
			id TEXT PRIMARY KEY,
			name TEXT,
			description TEXT,
			root_hash TEXT,
			created_at INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS bundles (
			id TEXT PRIMARY KEY,
			catalog_id TEXT,
			parent_bundle_id TEXT,
			type TEXT DEFAULT 'bundle',
			name TEXT,
			description TEXT,
			created_at INTEGER,
			FOREIGN KEY(catalog_id) REFERENCES catalogs(id),
			FOREIGN KEY(parent_bundle_id) REFERENCES bundles(id)
		);`,
		`CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			catalog_id TEXT,
			bundle_id TEXT,
			title TEXT,
			path TEXT,
			size INTEGER,
			chunk_hashes TEXT,
			source_url TEXT,
			FOREIGN KEY(catalog_id) REFERENCES catalogs(id),
			FOREIGN KEY(bundle_id) REFERENCES bundles(id)
		);`,
		`CREATE TABLE IF NOT EXISTS social_messages (
			id TEXT PRIMARY KEY,
			topic TEXT,
			author_id TEXT,
			content TEXT,
			ref_target_id TEXT, -- references catalog_id, file_id, or bundle_id
			created_at INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS metadata_versioning (
			target_id TEXT,
			key TEXT,
			value TEXT,
			version INTEGER,
			PRIMARY KEY(target_id, key, version)
		);`,
		`CREATE TABLE IF NOT EXISTS node_config (
			id INTEGER PRIMARY KEY,
			profile TEXT,
			cas_limit INTEGER,
			lora_port TEXT,
			lora_baud INTEGER,
			ble_enabled INTEGER
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("error executing query %q: %w", q, err)
		}
	}

	// Try to add new columns for older databases
	s.db.Exec("ALTER TABLE bundles ADD COLUMN type TEXT DEFAULT 'bundle'")
	s.db.Exec("ALTER TABLE files ADD COLUMN title TEXT DEFAULT ''")
	s.db.Exec("ALTER TABLE files ADD COLUMN source_url TEXT DEFAULT ''")
	s.db.Exec("ALTER TABLE node_config ADD COLUMN peer_retention_secs INTEGER DEFAULT 120")
	s.db.Exec("ALTER TABLE node_config ADD COLUMN auto_zoom_delay_secs INTEGER DEFAULT 60")
	s.db.Exec("ALTER TABLE node_config ADD COLUMN node_name TEXT DEFAULT ''")
	s.db.Exec("ALTER TABLE peers ADD COLUMN name TEXT DEFAULT ''")

	// Simple migrations for existing databases
	s.db.Exec(`ALTER TABLE files ADD COLUMN bundle_id TEXT REFERENCES bundles(id)`)
	s.db.Exec(`ALTER TABLE social_messages ADD COLUMN recipient_id TEXT DEFAULT ''`)

	return nil
}

type NodeConfig struct {
	NodeProfile   string `json:"node_profile"`
	CasLimitGB    int    `json:"cas_limit_gb"`
	LoraPort      string `json:"lora_port"`
	LoraBaud          int    `json:"lora_baud"`
	BleEnabled        bool   `json:"ble_enabled"`
	PeerRetention     int    `json:"peer_retention_secs"`
	AutoZoomDelaySecs int    `json:"auto_zoom_delay_secs"`
	NodeName          string `json:"node_name"`
}

func (s *Store) GetConfig() (*NodeConfig, error) {
	row := s.db.QueryRow("SELECT profile, cas_limit, lora_port, lora_baud, ble_enabled, COALESCE(peer_retention_secs, 120), COALESCE(auto_zoom_delay_secs, 60), COALESCE(node_name, '') FROM node_config WHERE id = 1")
	
	var conf NodeConfig
	var bleEnabledInt int
	err := row.Scan(&conf.NodeProfile, &conf.CasLimitGB, &conf.LoraPort, &conf.LoraBaud, &bleEnabledInt, &conf.PeerRetention, &conf.AutoZoomDelaySecs, &conf.NodeName)
	if err == sql.ErrNoRows {
		// Return default config
		return &NodeConfig{
			NodeProfile:       "hub",
			CasLimitGB:        50,
			LoraPort:          "/dev/ttyUSB0",
			LoraBaud:          115200,
			BleEnabled:        true,
			PeerRetention:     120,
			AutoZoomDelaySecs: 60,
			NodeName:          "",
		}, nil
	}
	if err != nil {
		return nil, err
	}
	
	conf.BleEnabled = bleEnabledInt == 1
	return &conf, nil
}

func (s *Store) SaveConfig(conf *NodeConfig) error {
	bleInt := 0
	if conf.BleEnabled {
		bleInt = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO node_config (id, profile, cas_limit, lora_port, lora_baud, ble_enabled, peer_retention_secs, auto_zoom_delay_secs, node_name)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			profile=excluded.profile,
			cas_limit=excluded.cas_limit,
			lora_port=excluded.lora_port,
			lora_baud=excluded.lora_baud,
			ble_enabled=excluded.ble_enabled,
			peer_retention_secs=excluded.peer_retention_secs,
			auto_zoom_delay_secs=excluded.auto_zoom_delay_secs,
			node_name=excluded.node_name
	`, conf.NodeProfile, conf.CasLimitGB, conf.LoraPort, conf.LoraBaud, bleInt, conf.PeerRetention, conf.AutoZoomDelaySecs, conf.NodeName)
	
	return err
}

type Catalog struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RootHash    string `json:"root_hash"`
	CreatedAt   int64  `json:"created_at"`
}

type Bundle struct {
	ID             string `json:"id"`
	CatalogID      string `json:"catalog_id"`
	ParentBundleID string `json:"parent_bundle_id"`
	Type           string `json:"type"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	CreatedAt      int64  `json:"created_at"`
}

type File struct {
	ID          string `json:"id"`
	CatalogID   string `json:"catalog_id"`
	BundleID    string `json:"bundle_id"`
	Title       string `json:"title"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ChunkHashes string `json:"chunk_hashes"` // JSON array of hash strings
	SourceURL   string `json:"source_url"`
}

func (s *Store) InsertCatalog(c *Catalog) error {
	_, err := s.db.Exec(`
		INSERT INTO catalogs (id, name, description, root_hash, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			description=excluded.description,
			root_hash=excluded.root_hash;
	`, c.ID, c.Name, c.Description, c.RootHash, c.CreatedAt)
	return err
}

func (s *Store) HasCatalog(id string) bool {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM catalogs WHERE id=?)", id).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

func (s *Store) GetCatalogs() ([]*Catalog, error) {
	rows, err := s.db.Query("SELECT id, name, description, root_hash, created_at FROM catalogs ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var catalogs []*Catalog
	for rows.Next() {
		var c Catalog
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.RootHash, &c.CreatedAt); err != nil {
			return nil, err
		}
		catalogs = append(catalogs, &c)
	}
	return catalogs, nil
}

func (s *Store) GetCatalogByID(id string) (*Catalog, error) {
	row := s.db.QueryRow("SELECT id, name, description, root_hash, created_at FROM catalogs WHERE id = ?", id)
	var c Catalog
	if err := row.Scan(&c.ID, &c.Name, &c.Description, &c.RootHash, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) InsertBundle(b *Bundle) error {
	_, err := s.db.Exec(`
		INSERT INTO bundles (id, catalog_id, parent_bundle_id, type, name, description, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type=excluded.type,
			name=excluded.name,
			description=excluded.description,
			parent_bundle_id=excluded.parent_bundle_id;
	`, b.ID, b.CatalogID, b.ParentBundleID, b.Type, b.Name, b.Description, b.CreatedAt)
	return err
}

func (s *Store) GetBundlesForCatalog(catalogID string) ([]*Bundle, error) {
	rows, err := s.db.Query("SELECT id, catalog_id, parent_bundle_id, COALESCE(type, 'bundle'), name, description, created_at FROM bundles WHERE catalog_id = ? AND (parent_bundle_id IS NULL OR parent_bundle_id = '') ORDER BY created_at DESC", catalogID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []*Bundle
	for rows.Next() {
		var b Bundle
		if err := rows.Scan(&b.ID, &b.CatalogID, &b.ParentBundleID, &b.Type, &b.Name, &b.Description, &b.CreatedAt); err != nil {
			return nil, err
		}
		bundles = append(bundles, &b)
	}
	return bundles, nil
}

func (s *Store) GetBundlesForBundle(bundleID string) ([]*Bundle, error) {
	rows, err := s.db.Query("SELECT id, catalog_id, parent_bundle_id, COALESCE(type, 'bundle'), name, description, created_at FROM bundles WHERE parent_bundle_id = ? ORDER BY created_at DESC", bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []*Bundle
	for rows.Next() {
		var b Bundle
		if err := rows.Scan(&b.ID, &b.CatalogID, &b.ParentBundleID, &b.Type, &b.Name, &b.Description, &b.CreatedAt); err != nil {
			return nil, err
		}
		bundles = append(bundles, &b)
	}
	return bundles, nil
}

func (s *Store) GetBundleByID(id string) (*Bundle, error) {
	row := s.db.QueryRow("SELECT id, catalog_id, parent_bundle_id, COALESCE(type, 'bundle'), name, description, created_at FROM bundles WHERE id = ?", id)
	var b Bundle
	if err := row.Scan(&b.ID, &b.CatalogID, &b.ParentBundleID, &b.Type, &b.Name, &b.Description, &b.CreatedAt); err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Store) InsertFile(f *File) error {
	_, err := s.db.Exec(`
		INSERT INTO files (id, catalog_id, bundle_id, title, path, size, chunk_hashes, source_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title,
			path=excluded.path,
			bundle_id=excluded.bundle_id,
			size=excluded.size,
			chunk_hashes=excluded.chunk_hashes,
			source_url=excluded.source_url;
	`, f.ID, f.CatalogID, f.BundleID, f.Title, f.Path, f.Size, f.ChunkHashes, f.SourceURL)
	return err
}

func (s *Store) GetFilesForCatalog(catalogID string) ([]*File, error) {
	rows, err := s.db.Query("SELECT id, catalog_id, COALESCE(bundle_id, ''), COALESCE(title, ''), path, size, chunk_hashes, COALESCE(source_url, '') FROM files WHERE catalog_id = ? AND (bundle_id IS NULL OR bundle_id = '')", catalogID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.CatalogID, &f.BundleID, &f.Title, &f.Path, &f.Size, &f.ChunkHashes, &f.SourceURL); err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	return files, nil
}

func (s *Store) GetFilesForBundle(bundleID string) ([]*File, error) {
	rows, err := s.db.Query("SELECT id, catalog_id, COALESCE(bundle_id, ''), COALESCE(title, ''), path, size, chunk_hashes, COALESCE(source_url, '') FROM files WHERE bundle_id = ?", bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.CatalogID, &f.BundleID, &f.Title, &f.Path, &f.Size, &f.ChunkHashes, &f.SourceURL); err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	return files, nil
}

func (s *Store) GetFileByID(fileID string) (*File, error) {
	row := s.db.QueryRow("SELECT id, catalog_id, COALESCE(bundle_id, ''), COALESCE(title, ''), path, size, chunk_hashes, COALESCE(source_url, '') FROM files WHERE id = ?", fileID)
	var f File
	if err := row.Scan(&f.ID, &f.CatalogID, &f.BundleID, &f.Title, &f.Path, &f.Size, &f.ChunkHashes, &f.SourceURL); err != nil {
		return nil, err
	}
	return &f, nil
}

type Peer struct {
	ID         string `json:"id"`
	Multiaddr  string `json:"multiaddr"`
	LastSeen   int64  `json:"last_seen"`
	TrustLevel int    `json:"trust_level"`
	Name       string `json:"name"`
}

func (s *Store) InsertPeer(p *Peer) error {
	_, err := s.db.Exec(`
		INSERT INTO peers (id, multiaddr, last_seen, trust_level, name)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			multiaddr=excluded.multiaddr,
			last_seen=excluded.last_seen,
			name=CASE WHEN excluded.name != '' THEN excluded.name ELSE name END;
	`, p.ID, p.Multiaddr, p.LastSeen, p.TrustLevel, p.Name)
	return err
}

func (s *Store) UpdatePeerName(id, name string) error {
	_, err := s.db.Exec("UPDATE peers SET name = ? WHERE id = ?", name, id)
	return err
}

func (s *Store) GetPeers() ([]*Peer, error) {
	rows, err := s.db.Query("SELECT id, multiaddr, last_seen, trust_level, COALESCE(name, '') FROM peers ORDER BY last_seen DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []*Peer
	for rows.Next() {
		var p Peer
		if err := rows.Scan(&p.ID, &p.Multiaddr, &p.LastSeen, &p.TrustLevel, &p.Name); err != nil {
			return nil, err
		}
		peers = append(peers, &p)
	}
	return peers, nil
}

type SocialMessage struct {
	ID          string `json:"id"`
	Topic       string `json:"topic"`
	AuthorID    string `json:"author_id"`
	RecipientID string `json:"recipient_id"` // Empty for public/topic messages
	Content     string `json:"content"`
	RefTargetID string `json:"ref_target_id"`
	CreatedAt   int64  `json:"created_at"`
}

func (s *Store) InsertSocialMessage(m *SocialMessage) error {
	if m.RecipientID == "" {
		m.RecipientID = "" // normalize
	}
	_, err := s.db.Exec(`
		INSERT INTO social_messages (id, topic, author_id, recipient_id, content, ref_target_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING;
	`, m.ID, m.Topic, m.AuthorID, m.RecipientID, m.Content, m.RefTargetID, m.CreatedAt)
	return err
}

func (s *Store) GetSocialMessages(topic string, limit int) ([]*SocialMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, topic, author_id, COALESCE(recipient_id, ''), content, ref_target_id, created_at 
		FROM social_messages 
		WHERE topic = ? AND (recipient_id IS NULL OR recipient_id = '')
		ORDER BY created_at ASC
		LIMIT ?
	`, topic, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*SocialMessage
	for rows.Next() {
		var m SocialMessage
		if err := rows.Scan(&m.ID, &m.Topic, &m.AuthorID, &m.RecipientID, &m.Content, &m.RefTargetID, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	return messages, nil
}

func (s *Store) GetSocialMessagesByRef(refTargetID string, limit int) ([]*SocialMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, topic, author_id, COALESCE(recipient_id, ''), content, ref_target_id, created_at 
		FROM social_messages 
		WHERE ref_target_id = ? 
		ORDER BY created_at ASC
		LIMIT ?
	`, refTargetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*SocialMessage
	for rows.Next() {
		var m SocialMessage
		if err := rows.Scan(&m.ID, &m.Topic, &m.AuthorID, &m.RecipientID, &m.Content, &m.RefTargetID, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	return messages, nil
}
