package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"resilient/internal/store"
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
)

const GlobalChatTopic = "/resilient/chat/global"
const DirectChatProtocolID = "/resilient/dm/1.0.0"

type ChatRoom struct {
	ctx   context.Context
	h     host.Host
	topic *pubsub.Topic
	sub   *pubsub.Subscription
	s     *store.Store
}

func setupChatRoom(ctx context.Context, h host.Host, ps *pubsub.PubSub, s *store.Store) (*ChatRoom, error) {
	topic, err := ps.Join(GlobalChatTopic)
	if err != nil {
		return nil, err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	cr := &ChatRoom{
		ctx:   ctx,
		h:     h,
		topic: topic,
		sub:   sub,
		s:     s,
	}

	h.SetStreamHandler(DirectChatProtocolID, cr.handleDirectMessageStream)

	go cr.readLoop()
	return cr, nil
}

func (cr *ChatRoom) handleDirectMessageStream(s network.Stream) {
	defer s.Close()
	var sm store.SocialMessage
	if err := json.NewDecoder(s).Decode(&sm); err != nil {
		log.Printf("Failed to decode direct message stream: %v", err)
		return
	}

	if err := cr.s.InsertSocialMessage(&sm); err != nil {
		log.Printf("Failed to save direct message: %v", err)
	}
}

func (cr *ChatRoom) readLoop() {
	for {
		msg, err := cr.sub.Next(cr.ctx)
		if err != nil {
			log.Printf("Chat readLoop exited: %v", err)
			return
		}

		// Don't process our own messages, we already saved them
		if msg.ReceivedFrom == cr.h.ID() {
			continue
		}

		var sm store.SocialMessage
		if err := json.Unmarshal(msg.Data, &sm); err != nil {
			log.Printf("Failed to unmarshal chat msg: %v", err)
			continue
		}

		// Save to db
		if err := cr.s.InsertSocialMessage(&sm); err != nil {
			log.Printf("Failed to save chat msg to db: %v", err)
		}
	}
}

func (cr *ChatRoom) Publish(content string, refTargetID string) (*store.SocialMessage, error) {
	sm := &store.SocialMessage{
		ID:          uuid.New().String(),
		Topic:       GlobalChatTopic,
		AuthorID:    cr.h.ID().String(),
		Content:     content,
		RefTargetID: refTargetID,
		CreatedAt:   time.Now().Unix(),
	}

	data, err := json.Marshal(sm)
	if err != nil {
		return nil, err
	}

	// Publish to network
	if err := cr.topic.Publish(cr.ctx, data); err != nil {
		return nil, err
	}

	// Save locally too
	if err := cr.s.InsertSocialMessage(sm); err != nil {
		log.Printf("Failed to save own message to db: %v", err)
	}

	return sm, nil
}

func (cr *ChatRoom) PublishDirect(content string, recipientID string) (*store.SocialMessage, error) {
	targetPeer, err := peer.Decode(recipientID)
	if err != nil {
		return nil, fmt.Errorf("invalid peer ID: %w", err)
	}

	sm := &store.SocialMessage{
		ID:          uuid.New().String(),
		Topic:       "direct",
		AuthorID:    cr.h.ID().String(),
		RecipientID: recipientID,
		Content:     content,
		CreatedAt:   time.Now().Unix(),
	}

	data, err := json.Marshal(sm)
	if err != nil {
		return nil, err
	}

	s, err := cr.h.NewStream(cr.ctx, targetPeer, DirectChatProtocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer for DM %s: %w", recipientID, err)
	}
	defer s.Close()

	if _, err := s.Write(data); err != nil {
		return nil, fmt.Errorf("failed to stream DM to peer %s: %w", recipientID, err)
	}

	if err := cr.s.InsertSocialMessage(sm); err != nil {
		log.Printf("Failed to save own DM to db: %v", err)
	}

	return sm, nil
}
