package daemon

import (
	"context"
	"encoding/json"
	"log"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"resilient/internal/store"
)

const CatalogAnnounceTopic = "/resilient/catalog/announce"

type CatalogAnnouncer struct {
	ctx   context.Context
	h     host.Host
	topic *pubsub.Topic
	sub   *pubsub.Subscription
	s     *store.Store
}

type CatalogBundle struct {
	Catalog *store.Catalog `json:"catalog"`
	Files   []*store.File  `json:"files"`
}

func setupCatalogAnnouncer(ctx context.Context, h host.Host, ps *pubsub.PubSub, s *store.Store) (*CatalogAnnouncer, error) {
	topic, err := ps.Join(CatalogAnnounceTopic)
	if err != nil {
		return nil, err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	ca := &CatalogAnnouncer{
		ctx:   ctx,
		h:     h,
		topic: topic,
		sub:   sub,
		s:     s,
	}

	go ca.readLoop()
	return ca, nil
}

func (ca *CatalogAnnouncer) readLoop() {
	for {
		msg, err := ca.sub.Next(ca.ctx)
		if err != nil {
			log.Printf("Catalog readLoop exited: %v", err)
			return
		}

		if msg.ReceivedFrom == ca.h.ID() {
			continue // ignore our own
		}

		var bndl CatalogBundle
		if err := json.Unmarshal(msg.Data, &bndl); err != nil {
			log.Printf("Failed to unmarshal catalog bundle: %v", err)
			continue
		}

		// Save the catalog and its files
		if bndl.Catalog != nil {
			isNew := !ca.s.HasCatalog(bndl.Catalog.ID)

			if err := ca.s.InsertCatalog(bndl.Catalog); err != nil {
				log.Printf("Failed to insert received catalog %s: %v", bndl.Catalog.ID, err)
			}
			for _, f := range bndl.Files {
				if err := ca.s.InsertFile(f); err != nil {
					log.Printf("Failed to insert received file %s: %v", f.ID, err)
				}
			}
			if isNew {
				log.Printf("Received & stored NEW catalog: %s from %s", bndl.Catalog.Name, msg.ReceivedFrom.String())
			}
		}
	}
}

// Announce broadcasts a given catalog and its files to the network
func (ca *CatalogAnnouncer) Announce(catalogID string) error {
	cats, err := ca.s.GetCatalogs()
	if err != nil {
		return err
	}
	var targetCat *store.Catalog
	for _, c := range cats {
		if c.ID == catalogID {
			targetCat = c
			break
		}
	}
	
	if targetCat == nil {
		return nil // Not found
	}

	files, err := ca.s.GetFilesForCatalog(catalogID)
	if err != nil {
		return err
	}

	bndl := CatalogBundle{
		Catalog: targetCat,
		Files:   files,
	}

	data, err := json.Marshal(bndl)
	if err != nil {
		return err
	}

	return ca.topic.Publish(ca.ctx, data)
}

// AnnounceAll safely broadcasts all catalogs we have to the network
func (ca *CatalogAnnouncer) AnnounceAll() {
	cats, err := ca.s.GetCatalogs()
	if err != nil {
		return
	}
	for _, c := range cats {
		if err := ca.Announce(c.ID); err != nil {
			log.Printf("Failed to announce catalog %s: %v", c.ID, err)
		}
	}
}
